package main

import (
	_ "embed"
	"strings"
)

//go:embed embed/architecture.md
var architectureTemplate string

//go:embed embed/agents-block.md
var agentsTemplate string

//go:embed embed/THIRD_PARTY_NOTICES.md
var thirdPartyNotices string

func architectureSkeleton() string {
	d := parseDocument(architectureTemplate)
	var b strings.Builder
	b.WriteString(strings.Join(d.lines[:d.sections[0].start], "\n") + "\n")
	for _, s := range d.sections {
		for _, def := range knownSections {
			if def.id == s.id && def.required {
				b.WriteString(strings.Join(d.lines[s.start:s.end], "\n") + "\n")
				break
			}
		}
	}
	return b.String()
}
