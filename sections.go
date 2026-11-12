package main

import (
	"strings"
)

type sectionDefinition struct {
	id, heading string
	required    bool
}

var knownSections = []sectionDefinition{
	{"project-structure", "Project layout", true},
	{"high-level-diagram", "System map", true},
	{"core-components", "Components", true},
	{"data-stores", "Data stores", false},
	{"external-integrations", "External services", false},
	{"deployment-infrastructure", "Build and deploy", false},
	{"decisions", "Key decisions", false},
	{"security-considerations", "Security", false},
	{"dev-testing-environment", "Development and testing", false},
	{"future-considerations", "Roadmap and known debt", false},
	{"project-identification", "About this project", true},
	{"glossary", "Glossary", false},
}

type section struct {
	id         string
	start, end int
}
type document struct {
	lines           []string
	sections        []section
	unknownHeadings []int
}

func normalize(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}
func linesOf(s string) []string {
	s = normalize(s)
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}
func markerID(line string) (string, bool) {
	const prefix, suffix = "<!-- arch:", " -->"
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, suffix) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(line, prefix), suffix)
	return id, id != "" && !strings.Contains(id, "-->")
}
func isH2(line string) bool {
	s := strings.TrimSpace(line)
	return s == "##" || strings.HasPrefix(s, "## ") || strings.HasPrefix(s, "##\t")
}
func parseDocument(s string) document {
	d := document{lines: linesOf(s)}
	var fence byte
	fenceLen := 0
	for i, line := range d.lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) >= 3 && (trimmed[0] == '`' || trimmed[0] == '~') {
			n := 0
			for n < len(trimmed) && trimmed[n] == trimmed[0] {
				n++
			}
			if fence == 0 && n >= 3 {
				fence, fenceLen = trimmed[0], n
				continue
			}
			if fence == trimmed[0] && n >= fenceLen && strings.TrimSpace(trimmed[n:]) == "" {
				fence = 0
				continue
			}
		}
		if fence != 0 {
			continue
		}
		if id, ok := markerID(line); ok {
			if len(d.sections) > 0 {
				d.sections[len(d.sections)-1].end = i
			}
			d.sections = append(d.sections, section{id, i, len(d.lines)})
			if i+1 == len(d.lines) || !isH2(d.lines[i+1]) {
				d.unknownHeadings = append(d.unknownHeadings, i+1)
			}
		} else if isH2(line) {
			if i == 0 {
				d.unknownHeadings = append(d.unknownHeadings, i+1)
			} else if _, ok := markerID(d.lines[i-1]); !ok {
				d.unknownHeadings = append(d.unknownHeadings, i+1)
			}
		}
	}
	return d
}
