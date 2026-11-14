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
	if !strings.HasPrefix(architectureTemplate, "<!-- archmd template v"+version+" | Section layout inspired by https://architecture.md/ -->\n# ") {
		t.Fatal("template preamble")
	}
	d := parseDocument(architectureTemplate)
	if len(d.sections) != len(knownSections) || len(d.unknownHeadings) > 0 {
		t.Fatal("template sections")
	}
	for i, def := range knownSections {
		s := d.sections[i]
		if s.id != def.id || d.lines[s.start+1] != "## "+def.heading || !strings.HasPrefix(d.lines[s.start+2], "<!-- hint: ") || !strings.HasSuffix(d.lines[s.start+2], " -->") {
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
			if !def.required && !strings.Contains(out, "INFO [unused-section] unused ID: "+def.id+"\n") {
				t.Fatal(out)
			}
		})
	}
}

func TestAttribution(t *testing.T) {
	config(t)
	const comment = "<!-- Section layout inspired by https://architecture.md/ -->"
	const notice = "節構成の発想は https://architecture.md/ (https://github.com/timajwilliams/architecture) に着想を得ている。文言・ヒント・指示文は独自に書いたもの。\n"
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
	for _, content := range []string{architectureTemplate, agentsTemplate, thirdPartyNotices, read(t, "README.md"), read(t, "ARCHITECTURE.md")} {
		for _, forbidden := range []string{"MIT", "Apache", "TODO"} {
			if strings.Contains(content, forbidden) {
				t.Errorf("出典に不要な文字列: %s", forbidden)
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
