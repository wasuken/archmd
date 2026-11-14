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

var sectionHints = map[string]string{
	"project-structure":         "Map the main directories to their responsibilities.",
	"high-level-diagram":        "Show the system overview and connections between major components.",
	"core-components":           "Describe each central module and the responsibility it owns.",
	"data-stores":               "Name persistent stores and their tables or collections without detailed schemas.",
	"external-integrations":     "Identify outside services and the reason each connection exists.",
	"deployment-infrastructure": "Outline where the system runs and how changes reach it.",
	"decisions":                 "Summarize each decision and its reason in one line.",
	"security-considerations":   "Summarize access boundaries and protections for sensitive information.",
	"dev-testing-environment":   "Name essential development tools and checks and link to setup instructions.",
	"future-considerations":     "Record likely architectural changes and unresolved design constraints.",
	"project-identification":    "Identify this project, its repository and maintainers, and its last reviewed date (YYYY-MM-DD).",
	"glossary":                  "Define local terminology that a new contributor needs to understand.",
}

func (def sectionDefinition) marker() string {
	return "<!-- arch:" + def.id + " | " + def.heading + " | " + sectionHints[def.id] + " -->"
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
	content := strings.TrimSuffix(strings.TrimPrefix(line, prefix), suffix)
	id, _, _ := strings.Cut(content, " | ")
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
		} else if isH2(line) {
			if i == 0 {
				d.unknownHeadings = append(d.unknownHeadings, i+1)
			} else {
				previous := i - 1
				for previous >= 0 && strings.TrimSpace(d.lines[previous]) == "" {
					previous--
				}
				if previous < 0 {
					d.unknownHeadings = append(d.unknownHeadings, i+1)
				} else if _, ok := markerID(d.lines[previous]); !ok {
					d.unknownHeadings = append(d.unknownHeadings, i+1)
				}
			}
		}
	}
	for _, section := range d.sections {
		for i := section.start + 1; i < section.end; i++ {
			if strings.TrimSpace(d.lines[i]) == "" {
				continue
			}
			if !isH2(d.lines[i]) {
				d.unknownHeadings = append(d.unknownHeadings, i+1)
			}
			break
		}
	}
	return d
}
