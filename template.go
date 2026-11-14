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

//go:embed NOTICE
var attributionNotices string

//go:embed LICENSE
var licenseText string

// バイナリ単体でも出典とライセンス全文を取り出せるようにする。
var thirdPartyNotices = attributionNotices + "\n" + licenseText

func architectureSkeleton() string {
	// 骨組みには従来の出典コメントを残す。
	d := parseDocument(strings.Replace(architectureTemplate, "archmd template v"+version+" | ", "", 1))
	var b strings.Builder
	b.WriteString(strings.Join(d.lines[:d.sections[0].start], "\n") + "\n")
	for _, def := range knownSections {
		b.WriteString(def.marker() + "\n")
		if def.required {
			b.WriteString("## " + def.heading + "\n")
		}
	}
	return b.String()
}
