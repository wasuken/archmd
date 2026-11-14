package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPhysicalLineCounts(t *testing.T) {
	for _, tc := range []struct {
		text  string
		count int
	}{{"", 0}, {"a", 1}, {"a\n", 1}, {"a\n\n", 2}, {"\n", 1}, {"a\r\nb\r\n", 2}, {"a\rb", 2}} {
		if got := len(linesOf(tc.text)); got != tc.count {
			t.Errorf("%q: %d != %d", tc.text, got, tc.count)
		}
	}
}
func TestTemplateStructure(t *testing.T) {
	if !strings.HasPrefix(architectureTemplate, "<!-- archmd template v"+version+" | Section layout inspired by https://architecture.md/ -->\n<!--") {
		t.Fatal("template preamble")
	}
	d := parseDocument(architectureTemplate)
	if len(d.sections) != len(knownSections) || len(d.unknownHeadings) > 0 {
		t.Fatal("template sections")
	}
	for i, def := range knownSections {
		s := d.sections[i]
		if s.id != def.id || d.lines[s.start+1] != "## "+def.heading || d.lines[s.start] != def.marker() {
			t.Errorf("bad section %s", def.id)
		}
	}
}

func TestSectionDefinitionsAndRequiredMissing(t *testing.T) {
	expected := []sectionDefinition{
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
	if len(knownSections) != len(expected) {
		t.Fatalf("section count: %d", len(knownSections))
	}
	d := parseDocument(architectureTemplate)
	for i, def := range expected {
		if knownSections[i] != def {
			t.Errorf("section %d: %+v, want %+v", i, knownSections[i], def)
		}
		t.Run(def.id, func(t *testing.T) {
			dir := baseProject(t)
			s := d.sections[i]
			text := append([]string{}, d.lines[:s.start]...)
			text = append(text, d.lines[s.end:]...)
			write(t, filepath.Join(dir, "ARCHITECTURE.md"), strings.Join(text, "\n")+"\n")
			code, out, stderr := invoke("check", "--dir", dir)
			if stderr != "" || (code == 1) != def.required || strings.Contains(out, "[required-missing]") != def.required {
				t.Fatalf("required check: %d %s %s", code, out, stderr)
			}
			if !def.required {
				for _, want := range []string{"WARN [missing-marker] missing ID " + def.id, def.marker(), "after known ID " + knownSections[i-1].id} {
					if !strings.Contains(out, want) {
						t.Fatalf("不足マーカーの追加案内がない: %s", out)
					}
				}
			}
		})
	}
}

func TestAttribution(t *testing.T) {
	config(t)
	const comment = "<!-- Section layout inspired by https://architecture.md/ -->"
	notice := read(t, "NOTICE") + "\n" + read(t, "LICENSE")
	if got := requireSuccess(t, "notices"); got != notice {
		t.Fatalf("notices: %q", got)
	}
	for _, name := range []string{"ARCHITECTURE.md", "AGENTS.md", "CLAUDE.md"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if name == "CLAUDE.md" {
				write(t, filepath.Join(dir, name), "existing instructions\n")
			}
			out := requireSuccess(t, "init", "--dir", dir)
			content := out
			if name != "CLAUDE.md" {
				content = read(t, filepath.Join(dir, name))
			}
			if !strings.Contains(content, comment+"\n") {
				t.Fatal("出典コメントがない")
			}
		})
	}
	for _, content := range []string{architectureTemplate, architectureSkeleton(), agentsTemplate} {
		for _, required := range []string{"timajwilliams", "SPDX-License-Identifier: Apache-2.0", "archmd により", "https://www.apache.org/licenses/LICENSE-2.0.txt"} {
			if !strings.Contains(content, required) {
				t.Errorf("出典・変更・ライセンス表示がない: %s", required)
			}
		}
	}
}
func TestParserSectionRangesAndFences(t *testing.T) {
	text := "# Header\n```md\n## Example\n<!-- arch:fake -->\n```\n<!-- arch:first -->\n## First\nbody\n\n<!-- arch:second -->\n## Second\n~~~\n<!-- arch:fake -->\n## Example\n~~~\n"
	d := parseDocument(text)
	if len(d.sections) != 2 || len(d.unknownHeadings) != 0 {
		t.Fatalf("%+v", d)
	}
	if d.sections[0].start != 5 || d.sections[0].end != 9 || d.sections[1].start != 9 || d.sections[1].end != 15 {
		t.Fatal(d.sections)
	}
}

func TestMarkerMetadataCompatibility(t *testing.T) {
	for _, tc := range []struct {
		line, id string
		valid    bool
	}{
		{"<!-- arch:custom -->", "custom", true},
		{"<!-- arch:custom | 見出し | hint | extra -->", "custom", true},
		{"<!-- arch:custom |  -->", "custom", true},
		{"<!-- arch:custom | heading --> inside | hint -->", "custom", true},
		{"<!-- arch: -->", "", false},
		{"<!-- arch: | heading | hint -->", "", false},
		{"<!-- arch:bad-->id | heading | hint -->", "", false},
		{" <!-- arch:custom -->", "", false},
		{"<!-- arch:custom --> ", "", false},
	} {
		id, ok := markerID(tc.line)
		if ok != tc.valid || ok && id != tc.id {
			t.Errorf("markerID(%q) = %q, %v", tc.line, id, ok)
		}
	}
}

func TestEmptySectionsAndTranslatedHeadings(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		var b strings.Builder
		for i, def := range knownSections {
			marker := def.marker()
			if legacy {
				marker = "<!-- arch:" + def.id + " -->"
			}
			b.WriteString(marker + "\n\n")
			if i%2 == 0 {
				b.WriteString("## 翻訳した見出し\n本文\n")
			}
		}
		dir := baseProject(t)
		write(t, filepath.Join(dir, "ARCHITECTURE.md"), b.String())
		out := requireSuccess(t, "check", "--dir", dir)
		want := "summary: 0 ERROR, 0 WARN, 0 INFO"
		if legacy {
			want = "summary: 0 ERROR, 1 WARN, 0 INFO"
		}
		if !strings.Contains(out, want) {
			t.Fatal(out)
		}
	}
	for _, text := range []string{
		"<!-- arch:custom -->\n\n本文\n",
		"<!-- arch:custom | Heading | Hint. -->\n\n本文\n",
	} {
		if got := parseDocument(text).unknownHeadings; len(got) != 1 || got[0] != 3 {
			t.Fatalf("最初の非空行の警告がない: %v", got)
		}
	}
}
