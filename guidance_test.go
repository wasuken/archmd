package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInitGuidancePreservesStdoutGolden(t *testing.T) {
	setTestVersion(t, "0.3.1")
	for _, common := range []bool{false, true} {
		for _, existingArch := range []bool{false, true} {
			for _, instructions := range []string{"none", "AGENTS.md", "CLAUDE.md", "existing-block"} {
				t.Run(fmt.Sprintf("common=%v/architecture=%v/%s", common, existingArch, instructions), func(t *testing.T) {
					conf := config(t)
					path := filepath.Join(conf, "architecture-md", "common.md")
					goldenName := "agents.golden"
					if common {
						if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
							t.Fatal(err)
						}
						write(t, path, "- Shared common rules.\r\n")
						goldenName = "agents-common.golden"
					}
					dir := t.TempDir()
					if existingArch {
						write(t, filepath.Join(dir, "ARCHITECTURE.md"), "# Existing architecture\n")
					}
					if instructions == "existing-block" {
						write(t, filepath.Join(dir, "AGENTS.md"), fixture(t, "agents.golden"))
					} else if instructions != "none" {
						write(t, filepath.Join(dir, instructions), "existing instructions\r\n")
					}
					code, out, stderr := invoke("init", "--dir", dir)
					if code != 0 {
						t.Fatalf("init: %d %s", code, stderr)
					}
					expectedStdout := ""
					if instructions == "AGENTS.md" || instructions == "CLAUDE.md" {
						expectedStdout = fixture(t, goldenName)
					}
					if out != expectedStdout {
						t.Fatalf("stdout differs from block golden:\n%q\nwant:\n%q", out, expectedStdout)
					}
					if instructions == "none" && read(t, filepath.Join(dir, "AGENTS.md")) != fixture(t, goldenName) {
						t.Fatal("written block differs from golden")
					}

					expectedTip := "Tip: shared rules (e.g. document language) can go in " + path + "\n"
					if common {
						expectedTip = "Using common rules from " + path + "\n"
					}
					if !strings.HasPrefix(stderr, expectedTip) || strings.Count(stderr, expectedTip) != 1 {
						t.Fatalf("common path guidance: %q", stderr)
					}
					if strings.Contains(stderr, "Next:") == existingArch {
						t.Fatalf("Next must appear only when architecture is created: %q", stderr)
					}
					const expectedNext = "Next: ARCHITECTURE.md is a skeleton. Have your coding agent fill it in:\n" +
						"  archmd prompt | <your-agent> && archmd check\n" +
						"  (agent examples: claude -p / codex exec -). Check the agent's docs for the flags that let it write files.\n"
					if !existingArch && (!strings.HasSuffix(stderr, expectedNext) || strings.Count(stderr, expectedNext) != 1) {
						t.Fatalf("Next is not last: %q", stderr)
					}
					if instructions == "AGENTS.md" || instructions == "CLAUDE.md" {
						if read(t, filepath.Join(dir, instructions)) != "existing instructions\r\n" {
							t.Fatal("existing instruction bytes changed")
						}
					}

					// 再実行ではファイルを変更せず、パス案内だけを繰り返す。
					before := snapshotPromptFiles(t, dir)
					secondCode, secondOut, secondErr := invoke("init", "--dir", dir)
					if secondCode != 0 || !strings.HasPrefix(secondErr, expectedTip) || strings.Contains(secondErr, "Next:") {
						t.Fatalf("second init: %d %q", secondCode, secondErr)
					}
					if secondOut != expectedStdout {
						t.Fatalf("second init stdout changed: %q", secondOut)
					}
					if !reflect.DeepEqual(before, snapshotPromptFiles(t, dir)) {
						t.Fatal("再実行でファイル、時刻またはディレクトリが変わった")
					}
				})
			}
		}
	}
}

func TestHelpConfigurationAndExamplesGolden(t *testing.T) {
	for _, common := range []bool{false, true} {
		t.Run(fmt.Sprintf("common=%v", common), func(t *testing.T) {
			conf := config(t)
			path := filepath.Join(conf, "architecture-md", "common.md")
			status := "not found"
			if common {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				write(t, path, "- Shared common rules.\n")
				status = "found"
			}
			expected := strings.NewReplacer("{{path}}", path, "{{status}}", status).Replace(fixture(t, "help.golden"))
			for _, args := range [][]string{{"help"}, {"--help"}, {"help", "--dir", t.TempDir()}} {
				code, out, stderr := invoke(args...)
				if code != 0 || stderr != "" || out != expected {
					t.Fatalf("%v: %d\n%s\n%s", args, code, out, stderr)
				}
				for _, forbidden := range []string{"-s", "workspace-write", "--permission"} {
					if strings.Contains(out, forbidden) {
						t.Errorf("help contains agent flag: %s", forbidden)
					}
				}
			}
			// The announced path and actual content source must agree for init and prompt.
			dir := t.TempDir()
			requireSuccess(t, "init", "--dir", dir)
			block := read(t, filepath.Join(dir, "AGENTS.md"))
			code, prompt, stderr := invoke("prompt", "--dir", dir)
			if code != 0 || stderr != "" || strings.Contains(prompt, path) || strings.Contains(prompt, "Tip: shared rules") || strings.Contains(prompt, "Using common rules from") || strings.Contains(prompt, "Next:") {
				t.Fatalf("prompt gained guidance: %d %q %q", code, prompt, stderr)
			}
			if strings.Contains(block, "Shared common rules.") != common || strings.Contains(prompt, "Shared common rules.") != common {
				t.Fatal("displayed common path does not match rules source")
			}
		})
	}
}

func TestInitFailureDoesNotPrintNext(t *testing.T) {
	conf := config(t)
	if err := os.MkdirAll(filepath.Join(conf, "architecture-md", "common.md"), 0755); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := invoke("init", "--dir", t.TempDir())
	if code != 1 || out != "" || strings.Contains(stderr, "Next:") {
		t.Fatalf("failed init: %d %q %q", code, out, stderr)
	}
}

func TestInitSkeletonAndLocalTemplate(t *testing.T) {
	config(t)
	dir := t.TempDir()
	code, out, stderr := invoke("init", "--dir", dir)
	if code != 0 || out != "" {
		t.Fatalf("init: %d %q %q", code, out, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "ARCHITECTURE.md.template")); !os.IsNotExist(err) {
		t.Fatal("テンプレートが作られた")
	}
	full := requireSuccess(t, "template")
	arch := read(t, filepath.Join(dir, "ARCHITECTURE.md"))
	d := parseDocument(arch)
	if len(d.sections) != 12 || len(d.unknownHeadings) != 0 {
		t.Fatal("骨組みの節数または見出しが不正")
	}
	for i, def := range knownSections {
		s := d.sections[i]
		if s.id != def.id || d.lines[s.start] != def.marker() {
			t.Fatalf("マーカーが不正: %s", def.id)
		}
		body := d.lines[s.start+1 : s.end]
		if def.required {
			if len(body) != 1 || body[0] != "## "+def.heading {
				t.Fatalf("必須節の骨組みが不正: %s", def.id)
			}
		} else if len(body) != 0 {
			t.Fatalf("空の節に本文がある: %s", def.id)
		}
	}
	for _, content := range []string{full, arch, read(t, filepath.Join(dir, "AGENTS.md"))} {
		if strings.Contains(content, "\r") || !strings.HasSuffix(content, "\n") {
			t.Fatal("生成物が LF 形式でない")
		}
	}
	if !strings.Contains(full, "last reviewed date (YYYY-MM-DD)") {
		t.Fatal("確認日のヒントがない")
	}
	requireSuccess(t, "check", "--dir", dir)
}

func TestInitPreservesExistingLocalTemplate(t *testing.T) {
	setTestVersion(t, "0.3.1")
	config(t)
	dir := baseProject(t)
	path := filepath.Join(dir, "ARCHITECTURE.md.template")
	const custom = "custom template\r\n\x00"
	write(t, path, custom)
	before := snapshotPromptFiles(t, dir)
	code, out, stderr := invoke("init", "--dir", dir)
	if code != 0 || out != fixture(t, "agents.golden") || strings.Contains(stderr, "ARCHITECTURE.md.template") {
		t.Fatalf("init: %d %q %q", code, out, stderr)
	}
	if !reflect.DeepEqual(before, snapshotPromptFiles(t, dir)) {
		t.Fatal("既存ファイルを変更した")
	}
}

func TestToolIndependentRules(t *testing.T) {
	config(t)
	dir := t.TempDir()
	requireSuccess(t, "init", "--dir", dir)
	block := read(t, filepath.Join(dir, "AGENTS.md"))
	prompt := requireSuccess(t, "prompt", "--dir", dir)
	for _, content := range []string{block, prompt} {
		for _, want := range []string{
			"When your change affects the project structure, components, data stores, external integrations, or deployment, update ARCHITECTURE.md in the same task.",
			"Never delete or edit the `<!-- arch:... -->` marker lines",
			"Only document what the repository evidences",
			"Not confirmed from the repository.",
			"verify that every component, service, data store, and integration",
			"each major source directory is covered",
			"if I need to change X, where do I look and what does it affect?",
			"If `archmd` is installed",
			"project-structure, high-level-diagram, core-components, project-identification",
			"If `.architecture/` exists (this requires `archmd`)",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("運用ルールがない: %s", want)
			}
		}
		if strings.Contains(content, "archmd template") || strings.Contains(content, "ARCHITECTURE.md.template") {
			t.Fatal("運用ルールが template コマンドに依存している")
		}
	}
	before := snapshotPromptFiles(t, dir)
	if second := requireSuccess(t, "prompt", "--dir", dir); second != prompt || !reflect.DeepEqual(before, snapshotPromptFiles(t, dir)) {
		t.Fatal("prompt が非決定的、またはファイルを変更した")
	}
}
