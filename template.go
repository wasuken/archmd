package main

import (
	_ "embed"
	"strings"
)

//go:embed embed/architecture.md
var architectureTemplateSource string

var architectureTemplate = strings.Replace(architectureTemplateSource, "{{VERSION}}", version, 1)

//go:embed embed/agents-block.md
var agentsTemplate string

//go:embed embed/THIRD_PARTY_NOTICES.md
var thirdPartyNotices string

func architectureSkeleton() string {
	// 骨組みには従来の出典コメントを残す。
	d := parseDocument(strings.Replace(architectureTemplate, "archmd template v"+version+" | ", "", 1))
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
