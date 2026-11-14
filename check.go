package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var templateVersionPattern = regexp.MustCompile(`archmd template v(\d+\.\d+\.\d+)(?:[ \t|]|$)`)
var blockVersionPattern = regexp.MustCompile(`<!-- archmd:start v(\d+\.\d+\.\d+) -->`)
var versionPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)

func parseVersion(value string) ([3]int, bool) {
	var parts [3]int
	match := versionPattern.FindStringSubmatch(value)
	if match == nil {
		return parts, false
	}
	for i := range parts {
		n, err := strconv.Atoi(match[i+1])
		if err != nil {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}

func isOlderVersion(stored, current string) bool {
	old, oldOK := parseVersion(stored)
	now, nowOK := parseVersion(current)
	if !oldOK || !nowOK {
		return false
	}
	for i := range old {
		if old[i] != now[i] {
			return old[i] < now[i]
		}
	}
	return false
}

func checkProject(dir string, maxLines, maxSectionLines int, out io.Writer) int {
	errors, warnings, infos := 0, 0, 0
	report := func(level, code, message string) {
		fmt.Fprintf(out, "%s [%s] %s\n", level, code, message)
		switch level {
		case "ERROR":
			errors++
		case "WARN":
			warnings++
		case "INFO":
			infos++
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "ARCHITECTURE.md"))
	lineCount := 0
	if err != nil {
		if os.IsNotExist(err) {
			report("ERROR", "no-architecture-file", "ARCHITECTURE.md does not exist")
		} else {
			report("ERROR", "io", err.Error())
		}
	} else {
		d := parseDocument(string(data))
		lineCount = len(d.lines) - len(d.sections)
		known := map[string]bool{}
		for _, def := range knownSections {
			known[def.id] = true
		}
		counts := map[string]int{}
		for _, s := range d.sections {
			counts[s.id]++
			if counts[s.id] == 2 {
				report("ERROR", "duplicate-id", "duplicate ID: "+s.id)
			}
			if !known[s.id] {
				report("WARN", "unknown-id", "unknown ID: "+s.id)
			}
			if s.end-s.start-1 > maxSectionLines {
				report("WARN", "section-lines", fmt.Sprintf("%s: %d lines exceeds limit %d", s.id, s.end-s.start-1, maxSectionLines))
			}
		}
		for i, def := range knownSections {
			if counts[def.id] == 0 {
				if def.required {
					report("ERROR", "required-missing", "missing required ID: "+def.id)
				} else {
					report("WARN", "missing-marker", fmt.Sprintf("missing ID %s; add %s after known ID %s", def.id, def.marker(), knownSections[i-1].id))
				}
			}
		}
		for _, line := range d.unknownHeadings {
			report("WARN", "unknown-heading", fmt.Sprintf("line %d: H2 without a marker or first nonblank line after marker is not H2", line))
		}
		if maxLines > 0 && lineCount > maxLines {
			report("ERROR", "max-lines", fmt.Sprintf("current %d lines, limit %d, excess %d", lineCount, maxLines, lineCount-maxLines))
		}
		for _, s := range d.sections {
			if !strings.Contains(d.lines[s.start], " | ") {
				report("WARN", "legacy-format", "ARCHITECTURE.md uses the pre-0.2.0 format. Regenerate it: archmd remove --yes && archmd init --write-agents && archmd prompt | <agent>")
				break
			}
		}
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".architecture")); statErr == nil {
		expected, buildErr := buildContent(dir)
		if buildErr != nil {
			report("ERROR", "io", buildErr.Error())
		} else if !bytes.Equal(data, expected) {
			report("ERROR", "drift", "ARCHITECTURE.md differs from .architecture/; run archmd build")
		}
	} else if !os.IsNotExist(statErr) {
		report("ERROR", "io", statErr.Error())
	}
	templatePath := filepath.Join(dir, "ARCHITECTURE.md.template")
	if _, err := os.Lstat(templatePath); err == nil {
		report("WARN", "legacy-template", "ARCHITECTURE.md.template is no longer used since v0.2.0. Delete it manually or run: archmd remove --yes")
	}
	if template, err := os.ReadFile(templatePath); err == nil {
		firstLine, _, _ := strings.Cut(string(template), "\n")
		match := templateVersionPattern.FindStringSubmatch(strings.TrimSuffix(firstLine, "\r"))
		if match != nil && isOlderVersion(match[1], version) {
			report("INFO", "outdated-template", fmt.Sprintf("ARCHITECTURE.md.template is v%s; this archmd is v%s", match[1], version))
		}
	}
	instructions, agentName, agentsErr := preferredAgentInstructions(dir)
	if os.IsNotExist(agentsErr) {
		report("WARN", "no-agents-file", "neither AGENTS.md nor CLAUDE.md exists")
	} else if agentsErr != nil {
		report("ERROR", "io", agentsErr.Error())
	} else if !hasArchitectureReference(instructions) {
		report("WARN", "no-reference", "agent instructions do not mention ARCHITECTURE.md")
	}
	if agentsErr == nil {
		match := blockVersionPattern.FindSubmatch(instructions)
		if match != nil && isOlderVersion(string(match[1]), version) {
			report("INFO", "outdated-block", fmt.Sprintf("archmd block in %s is v%s; this archmd is v%s", agentName, match[1], version))
		}
	}
	fmt.Fprintf(out, "lines: %d\nsummary: %d ERROR, %d WARN, %d INFO\n", lineCount, errors, warnings, infos)
	if errors > 0 {
		return 1
	}
	return 0
}
