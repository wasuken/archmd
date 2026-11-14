package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRemoveDryRunAndCommonUntouched(t *testing.T) {
	for _, keep := range []bool{false, true} {
		t.Run(map[bool]string{false: "delete", true: "keep"}[keep], func(t *testing.T) {
			conf := config(t)
			folder := filepath.Join(conf, "architecture-md")
			if err := os.MkdirAll(folder, 0755); err != nil {
				t.Fatal(err)
			}
			commonPath := filepath.Join(folder, "common.md")
			write(t, commonPath, "- Common rules must survive removal.\r\n")
			dir := t.TempDir()
			requireSuccess(t, "init", "--dir", dir)
			requireSuccess(t, "split", "--dir", dir)
			write(t, filepath.Join(dir, "ARCHITECTURE.md.template"), "custom template\r\n\x00")
			before, configBefore := snapshotPromptFiles(t, dir), snapshotPromptFiles(t, conf)
			args := []string{"remove", "--dir", dir}
			if keep {
				args = append(args, "--keep-architecture-md")
			}
			code, out, stderr := invoke(args...)
			if code != 0 || stderr != "" || !strings.HasSuffix(out, "NOTHING WAS REMOVED. Re-run with --yes to apply.\n") {
				t.Fatalf("dry-run: %d %q %q", code, out, stderr)
			}
			for _, text := range []string{"ARCHITECTURE.md:", "ARCHITECTURE.md.template: would remove", ".architecture:", "AGENTS.md:", "手で書き足した", "再生成でき", "復元できない"} {
				if !strings.Contains(out, text) {
					t.Errorf("missing preview or warning %q", text)
				}
			}
			if !reflect.DeepEqual(before, snapshotPromptFiles(t, dir)) || !reflect.DeepEqual(configBefore, snapshotPromptFiles(t, conf)) {
				t.Fatal("dry-run changed directory entries, contents, modes, or timestamps")
			}
			code, out, stderr = invoke(append(args, "--yes")...)
			if code != 0 || stderr != "" || !strings.Contains(out, "手で書き足した") || !strings.Contains(out, "再生成でき") || !strings.Contains(out, "復元できない") {
				t.Fatalf("apply warnings: %d %q %q", code, out, stderr)
			}
			if !reflect.DeepEqual(configBefore, snapshotPromptFiles(t, conf)) {
				t.Fatal("apply touched user configuration")
			}
			if _, err := os.Stat(filepath.Join(dir, "ARCHITECTURE.md.template")); !os.IsNotExist(err) {
				t.Fatal("template was not removed", err)
			}
		})
	}
}

func TestInitRemoveDeletesLocalTemplate(t *testing.T) {
	config(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "ARCHITECTURE.md.template")
	write(t, path, "custom template\r\n\x00")
	before := snapshotPromptFiles(t, dir)["ARCHITECTURE.md.template"]
	requireSuccess(t, "init", "--dir", dir)
	if before != snapshotPromptFiles(t, dir)["ARCHITECTURE.md.template"] {
		t.Fatal("init changed the template")
	}
	out := requireSuccess(t, "remove", "--yes", "--dir", dir)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("unexpected remaining files: %v %v", entries, err)
	}
	for _, result := range []string{"ARCHITECTURE.md: removed", "ARCHITECTURE.md.template: removed", ".architecture: skipped(not found)", "AGENTS.md: removed"} {
		if !strings.Contains(out, result) {
			t.Error(out)
		}
	}
	if strings.Index(out, "WARNING:") > strings.Index(out, "ARCHITECTURE.md: removed") {
		t.Fatal("warnings must precede removal results")
	}
}

func TestRemovePreservesOutsideBytes(t *testing.T) {
	block := "<!-- archmd:start v0.1.0 -->\r\nhandwritten text\n<!-- archmd:end -->\r\n"
	for _, tc := range []struct {
		name, original, expected string
	}{
		{"mixed-newlines", "prefix\r\n\n" + block + "\r\n\r\nsuffix\x00", "prefix\r\n\n\r\nsuffix\x00"},
		{"one-blank", "prefix\n" + block + "\n\n\nend\n", "prefix\n\n\nend\n"},
		{"no-blank", "prefix\n" + block + "end", "prefix\nend"},
		{"spaces-blank", "prefix\r" + block + " \t\r\nend\r", "prefix\rend\r"},
		{"multiple", "prefix\n" + block + "\nmid\n" + block + "\nend", "prefix\nmid\nend"},
		{"no-markers", " \t\r\n", " \t\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "AGENTS.md")
			write(t, path, tc.original)
			requireSuccess(t, "remove", "--yes", "--dir", dir)
			if got := read(t, path); got != tc.expected {
				t.Fatalf("outside bytes changed: %q != %q", got, tc.expected)
			}
		})
	}
}

func TestRemoveSplitAndKeepArchitecture(t *testing.T) {
	for _, keep := range []bool{false, true} {
		t.Run(map[bool]string{false: "delete", true: "keep"}[keep], func(t *testing.T) {
			config(t)
			dir := t.TempDir()
			requireSuccess(t, "init", "--dir", dir)
			requireSuccess(t, "split", "--dir", dir)
			// Every descendant, including non-Markdown content, belongs to the removed directory.
			nested := filepath.Join(dir, ".architecture", "nested")
			if err := os.Mkdir(nested, 0755); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(nested, "notes.txt"), "notes")
			path := filepath.Join(dir, "ARCHITECTURE.md")
			before := read(t, path)
			args := []string{"remove", "--yes", "--dir", dir}
			if keep {
				args = append(args, "--keep-architecture-md")
			}
			out := requireSuccess(t, args...)
			if keep {
				if got := read(t, path); got != strings.TrimPrefix(before, generatedHeader+"\n\n") || strings.Contains(got, generatedHeader) {
					t.Fatal("architecture not restored to a normal document")
				}
				if !strings.Contains(out, "ARCHITECTURE.md: kept") {
					t.Fatal(out)
				}
			} else if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("architecture was not removed")
			}
			for _, name := range []string{".architecture", "AGENTS.md"} {
				if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
					t.Fatalf("%s was not removed", name)
				}
			}
		})
	}
}

func TestRemoveInvalidMarkersChangesNothing(t *testing.T) {
	for _, markers := range []string{
		"<!-- archmd:start v0.1.0 -->\ntext\n",
		"text\n<!-- archmd:end -->\n",
		"<!-- archmd:end -->\n<!-- archmd:start v0.1.0 -->\n",
		"<!-- archmd:start -->\n<!-- archmd:start -->\n<!-- archmd:end -->\n",
		"<!-- archmd:start -->\n<!-- archmd:end -->\n<!-- archmd:start -->\n",
	} {
		for _, yes := range []bool{false, true} {
			dir := baseProject(t)
			requireSuccess(t, "split", "--dir", dir)
			write(t, filepath.Join(dir, "AGENTS.md"), markers)
			write(t, filepath.Join(dir, "ARCHITECTURE.md.template"), "custom template\n")
			before := snapshotPromptFiles(t, dir)
			args := []string{"remove", "--dir", dir}
			if yes {
				args = append(args, "--yes")
			}
			code, _, stderr := invoke(args...)
			if code != 1 || !strings.Contains(stderr, "marker") {
				t.Fatalf("malformed block accepted: %d %q", code, stderr)
			}
			if !reflect.DeepEqual(before, snapshotPromptFiles(t, dir)) {
				t.Fatal("malformed markers caused partial removal")
			}
		}
	}
}

func TestRemovePreferredInstructionsAndMissingTargets(t *testing.T) {
	dir := t.TempDir()
	block := "<!-- archmd:start v0.1.0 -->\ncustom rule\n<!-- archmd:end -->\n"
	write(t, filepath.Join(dir, "CLAUDE.md"), "before\n"+block+"\nafter\n")
	write(t, filepath.Join(dir, "AGENTS.md"), "no block\r\n")
	beforeClaude := read(t, filepath.Join(dir, "CLAUDE.md"))
	requireSuccess(t, "remove", "--yes", "--dir", dir)
	if read(t, filepath.Join(dir, "CLAUDE.md")) != beforeClaude || read(t, filepath.Join(dir, "AGENTS.md")) != "no block\r\n" {
		t.Fatal("AGENTS.md was not preferred")
	}
	if err := os.Remove(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	out := requireSuccess(t, "remove", "--yes", "--dir", dir)
	if read(t, filepath.Join(dir, "CLAUDE.md")) != "before\nafter\n" || !strings.Contains(out, "CLAUDE.md: removed (archmd block)") {
		t.Fatal("CLAUDE.md fallback failed", out)
	}
	empty := t.TempDir()
	out = requireSuccess(t, "--dir", empty, "remove", "--yes")
	if strings.Count(out, "skipped(not found)") != 4 {
		t.Fatal(out)
	}
}

func TestRemoveGitDirectoryWarning(t *testing.T) {
	dir := t.TempDir()
	out := requireSuccess(t, "remove", "--dir", dir)
	if !strings.Contains(out, "git管理外のため復元できない") {
		t.Fatal("missing non-git warning")
	}
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	out = requireSuccess(t, "remove", "--yes", "--dir", dir)
	if strings.Contains(out, "git管理外") {
		t.Fatal("unexpected non-git warning")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatal(".git was modified", err)
	}
}

func TestRemoveKeepArchitectureBytes(t *testing.T) {
	for _, tc := range []struct{ input, expected string }{
		{"# Plain\r\n\r\nbody", "# Plain\r\n\r\nbody"},
		{generatedHeader + "\r\n\r\n\r\nbody\r\n", "\r\nbody\r\n"},
		{generatedHeader + "\nbody", "body"},
		{generatedHeader, ""},
		{"prefix\n" + generatedHeader + "\n", "prefix\n" + generatedHeader + "\n"},
	} {
		dir := t.TempDir()
		path := filepath.Join(dir, "ARCHITECTURE.md")
		write(t, path, tc.input)
		requireSuccess(t, "remove", "--yes", "--keep-architecture-md", "--dir", dir)
		if got := read(t, path); got != tc.expected {
			t.Fatalf("keep bytes: %q != %q", got, tc.expected)
		}
	}
}

func TestRemoveRejectsSymlinksBeforeChanges(t *testing.T) {
	for _, target := range []string{"ARCHITECTURE.md", "ARCHITECTURE.md.template", ".architecture", "AGENTS.md"} {
		t.Run(target, func(t *testing.T) {
			dir, outside := t.TempDir(), t.TempDir()
			write(t, filepath.Join(outside, "common.md"), "outside data\n")
			linkTarget := filepath.Join(outside, "common.md")
			if target == ".architecture" {
				linkTarget = outside
			}
			if err := os.Symlink(linkTarget, filepath.Join(dir, target)); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			if target != "ARCHITECTURE.md" {
				write(t, filepath.Join(dir, "ARCHITECTURE.md"), "keep me\n")
			}
			before := snapshotPromptFiles(t, outside)
			code, _, _ := invoke("remove", "--yes", "--dir", dir)
			if code != 1 || !reflect.DeepEqual(before, snapshotPromptFiles(t, outside)) {
				t.Fatal("symlink target was modified or accepted")
			}
			if _, err := os.Lstat(filepath.Join(dir, target)); err != nil {
				t.Fatal("symlink was removed")
			}
			if target != "ARCHITECTURE.md" && read(t, filepath.Join(dir, "ARCHITECTURE.md")) != "keep me\n" {
				t.Fatal("preflight allowed partial removal")
			}
		})
	}
}

func TestRemoveArgumentErrors(t *testing.T) {
	for _, args := range [][]string{{"remove", "--yes=bad"}, {"remove", "--keep-architecture-md=bad"}, {"remove", "--max-lines", "300"}, {"remove", "extra"}} {
		code, _, _ := invoke(args...)
		if code != 2 {
			t.Errorf("%v: %d", args, code)
		}
	}
}
