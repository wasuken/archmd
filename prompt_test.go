package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type promptFileState struct {
	Content  string
	Mode     fs.FileMode
	Modified time.Time
}

func snapshotPromptFiles(t *testing.T, root string) map[string]promptFileState {
	t.Helper()
	files := make(map[string]promptFileState)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		state := promptFileState{Mode: info.Mode(), Modified: info.ModTime()}
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			state.Content = string(data)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[relative] = state
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestPromptModesDeterminismAndReadOnly(t *testing.T) {
	setTestVersion(t, "0.3.1")
	for _, mode := range []string{"direct", "split", "missing", "incomplete", "drift"} {
		for _, common := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/common=%v", mode, common), func(t *testing.T) {
				conf := config(t)
				if common {
					folder := filepath.Join(conf, "architecture-md")
					if err := os.MkdirAll(folder, 0755); err != nil {
						t.Fatal(err)
					}
					write(t, filepath.Join(folder, "common.md"), "- Follow the shared repository conventions.\r\n")
				}
				dir := baseProject(t)
				switch mode {
				case "missing":
					if err := os.Remove(filepath.Join(dir, "ARCHITECTURE.md")); err != nil {
						t.Fatal(err)
					}
				case "incomplete":
					write(t, filepath.Join(dir, "ARCHITECTURE.md"), "# Incomplete\n## Unmarked\n")
				case "split", "drift":
					requireSuccess(t, "split", "--dir", dir)
					if mode == "drift" {
						path := filepath.Join(dir, "ARCHITECTURE.md")
						write(t, path, read(t, path)+"## Manual change\n")
					}
				}
				before, configBefore := snapshotPromptFiles(t, dir), snapshotPromptFiles(t, conf)
				code, out, stderr := invoke("prompt", "--dir", dir)
				if code != 0 || stderr != "" {
					t.Fatalf("prompt: %d %q %q", code, out, stderr)
				}
				secondCode, secondOut, secondErr := invoke("prompt", "--dir", dir)
				if code != secondCode || out != secondOut || stderr != secondErr {
					t.Fatal("prompt is not deterministic")
				}
				if !reflect.DeepEqual(before, snapshotPromptFiles(t, dir)) || !reflect.DeepEqual(configBefore, snapshotPromptFiles(t, conf)) {
					t.Fatal("prompt changed files, modes, timestamps, or directory entries")
				}

				instruction := "Edit only the affected sections of ARCHITECTURE.md directly.\n\n"
				if mode == "split" || mode == "drift" {
					instruction = "Edit the files in `.architecture/`, then run `archmd build`. Do not edit ARCHITECTURE.md directly.\n\n"
				}
				block, err := agentsBlock(0)
				if err != nil {
					t.Fatal(err)
				}
				rules := strings.TrimSuffix(strings.TrimPrefix(block, "<!-- archmd:start v"+version+" -->\n"), "<!-- archmd:end -->\n")
				_, diagnostics, _ := invoke("check", "--dir", dir)
				var kept []string
				for _, line := range strings.Split(diagnostics, "\n") {
					if !strings.HasPrefix(line, "WARN [legacy-format] ") && !strings.HasPrefix(line, "WARN [legacy-template] ") {
						kept = append(kept, line)
					}
				}
				diagnostics = strings.Join(kept, "\n")
				summaryIndex := strings.Index(diagnostics, "summary:")
				if summaryIndex < 0 {
					t.Fatal("check summary missing")
				}
				expected := "Update ARCHITECTURE.md so it reflects the current state of this codebase.\n\n" + instruction + rules + "\n" + diagnostics[:summaryIndex]
				if mode == "missing" {
					expected += "Run `archmd init` first to create ARCHITECTURE.md.\n"
				}
				expected += "\nYou are done when `archmd check` exits with code 0.\n"
				if out != expected {
					t.Fatalf("prompt content or ordering differs:\n%s\nexpected:\n%s", out, expected)
				}
				if strings.Contains(out, "archmd:start") || strings.Contains(out, "archmd:end") || strings.Contains(out, "\r") {
					t.Fatal("unexpected marker or CR in prompt")
				}
				if strings.Contains(out, "Follow the shared repository conventions.") != common {
					t.Fatal("common.md inclusion differs")
				}
				switch mode {
				case "missing":
					if !strings.Contains(out, "ERROR [no-architecture-file]") {
						t.Fatal(out)
					}
				case "incomplete":
					if !strings.Contains(out, "ERROR [required-missing]") || !strings.Contains(out, "WARN [unknown-heading]") {
						t.Fatal(out)
					}
				case "drift":
					if !strings.Contains(out, "ERROR [drift]") || !strings.Contains(out, "WARN [unknown-heading]") {
						t.Fatal(out)
					}
				}
			})
		}
	}
}

func TestPromptMaxLines(t *testing.T) {
	setTestVersion(t, "0.3.1")
	config(t)
	dir := baseProject(t)
	path := filepath.Join(dir, "ARCHITECTURE.md")
	write(t, path, read(t, path)+strings.Repeat("body\n", 288))
	code, out, stderr := invoke("prompt", "--max-lines", "300", "--dir", dir)
	if code != 0 || stderr != "" {
		t.Fatalf("%d %s %s", code, out, stderr)
	}
	for _, want := range []string{
		"within 300 lines", "archmd check --max-lines 300",
		"detailed schemas, Glossary, Roadmap, then long setup instructions",
		"Preserve Project layout, System map, and Components",
		"ERROR [max-lines] current 301 lines, limit 300, excess 1", "lines: 301\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	block, err := agentsBlock(300)
	if err != nil {
		t.Fatal(err)
	}
	rules := strings.TrimSuffix(strings.TrimPrefix(block, "<!-- archmd:start v"+version+" -->\n"), "<!-- archmd:end -->\n")
	if !strings.Contains(out, rules) {
		t.Fatal("prompt and init rules differ with --max-lines")
	}
}

func TestPromptArgumentsAndCommonError(t *testing.T) {
	conf := config(t)
	dir := baseProject(t)
	requireSuccess(t, "--dir", dir, "prompt")
	for _, args := range [][]string{
		{"--max-lines", "0"}, {"--max-lines", "-1"}, {"--max-lines", "bad"},
		{"--max-section-lines", "5"}, {"extra"},
	} {
		code, out, _ := invoke(append([]string{"prompt", "--dir", dir}, args...)...)
		if code != 2 || out != "" {
			t.Errorf("%v: got %d %q", args, code, out)
		}
	}
	if err := os.MkdirAll(filepath.Join(conf, "architecture-md", "common.md"), 0755); err != nil {
		t.Fatal(err)
	}
	before := snapshotPromptFiles(t, dir)
	code, out, stderr := invoke("prompt", "--dir", dir)
	if code != 1 || out != "" || stderr == "" {
		t.Fatalf("common read error: %d %q %q", code, out, stderr)
	}
	if !reflect.DeepEqual(before, snapshotPromptFiles(t, dir)) {
		t.Fatal("failed prompt changed files")
	}
}

func TestPromptOmitsLegacyWarnings(t *testing.T) {
	setTestVersion(t, "0.3.1")
	config(t)
	dir := baseProject(t)
	write(t, filepath.Join(dir, "ARCHITECTURE.md.template"), "<!-- archmd template v0.1.0 -->\n")
	check := requireSuccess(t, "check", "--dir", dir)
	for _, want := range []string{"WARN [legacy-format]", "WARN [legacy-template]", "INFO [outdated-template]", "WARN [missing-marker]"} {
		if !strings.Contains(check, want) {
			t.Fatalf("check に診断がない: %s", want)
		}
	}
	before := snapshotPromptFiles(t, dir)
	prompt := requireSuccess(t, "prompt", "--dir", dir)
	for _, unwanted := range []string{"legacy-format", "legacy-template", "archmd remove", "Delete it manually or run:", "Regenerate it:"} {
		if strings.Contains(prompt, unwanted) {
			t.Errorf("prompt に除外対象が含まれる: %s", unwanted)
		}
	}
	for _, want := range []string{"INFO [outdated-template]", "WARN [missing-marker]", "lines: 13\n"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt から必要な診断が消えた: %s", want)
		}
	}
	if !reflect.DeepEqual(before, snapshotPromptFiles(t, dir)) {
		t.Fatal("prompt が既存ファイルを変更した")
	}
}
