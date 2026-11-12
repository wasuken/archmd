package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

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
		lineCount = len(d.lines)
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
			if s.end-s.start > maxSectionLines {
				report("WARN", "section-lines", fmt.Sprintf("%s: %d lines exceeds limit %d", s.id, s.end-s.start, maxSectionLines))
			}
		}
		for _, def := range knownSections {
			if counts[def.id] == 0 {
				if def.required {
					report("ERROR", "required-missing", "missing required ID: "+def.id)
				} else {
					report("INFO", "unused-section", "unused ID: "+def.id)
				}
			}
		}
		for _, line := range d.unknownHeadings {
			report("WARN", "unknown-heading", fmt.Sprintf("line %d: H2 without a marker or marker not immediately followed by H2", line))
		}
		if maxLines > 0 && lineCount > maxLines {
			report("ERROR", "max-lines", fmt.Sprintf("current %d lines, limit %d, excess %d", lineCount, maxLines, lineCount-maxLines))
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
	instructions, agentsErr := preferredAgentInstructions(dir)
	if os.IsNotExist(agentsErr) {
		report("WARN", "no-agents-file", "neither AGENTS.md nor CLAUDE.md exists")
	} else if agentsErr != nil {
		report("ERROR", "io", agentsErr.Error())
	} else if !hasArchitectureReference(instructions) {
		report("WARN", "no-reference", "agent instructions do not mention ARCHITECTURE.md")
	}
	fmt.Fprintf(out, "lines: %d\nsummary: %d ERROR, %d WARN, %d INFO\n", lineCount, errors, warnings, infos)
	if errors > 0 {
		return 1
	}
	return 0
}
