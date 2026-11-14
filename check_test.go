package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsOlderVersion(t *testing.T) {
	for _, tc := range []struct {
		stored, current string
		want            bool
	}{
		{"0.0.9", "0.1.0", true},
		{"0.1.0", "0.1.0", false},
		{"0.1.1", "0.1.0", false},
		{"0.9.0", "0.10.0", true},
		{"0.10.0", "0.9.0", false},
		{"0.99.99", "1.0.0", true},
		{"1.0.0", "0.99.99", false},
		{"1.2.9", "1.2.10", true},
		{"1.2.10", "1.2.9", false},
		{"0.01.0", "0.1.0", false},
		{"", "0.1.0", false},
		{"0.0", "0.1.0", false},
		{"0.0.0.0", "0.1.0", false},
		{"-1.0.0", "0.1.0", false},
		{"v0.0.0", "0.1.0", false},
		{"0.x.0", "0.1.0", false},
		{"0.0.0", "invalid", false},
		{"999999999999999999999999999999.0.0", "1.0.0", false},
		{"0.0.0", "999999999999999999999999999999.0.0", false},
	} {
		t.Run(tc.stored+"/"+tc.current, func(t *testing.T) {
			if got := isOlderVersion(tc.stored, tc.current); got != tc.want {
				t.Fatalf("isOlderVersion(%q, %q) = %v, want %v", tc.stored, tc.current, got, tc.want)
			}
		})
	}
}

func TestCheckOutdatedTemplate(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		outdated      bool
	}{
		{"missing", "", false},
		{"empty", "", false},
		{"unstamped", "<!-- Section layout inspired by https://architecture.md/ -->\n", false},
		{"old", "<!-- archmd template v0.0.9 | Section layout inspired by https://architecture.md/ -->\n", true},
		{"same", "<!-- archmd template v" + version + " | Section layout inspired by https://architecture.md/ -->\n", false},
		{"newer", "<!-- archmd template v1.0.0 | Section layout inspired by https://architecture.md/ -->\n", false},
		{"invalid", "<!-- archmd template v0.x.0 -->\n", false},
		{"incomplete", "<!-- archmd template v0.0 -->\n", false},
		{"extra-component", "<!-- archmd template v0.0.0.0 -->\n", false},
		{"suffix", "<!-- archmd template v0.0.0oops -->\n", false},
		{"overflow", "<!-- archmd template v0.0.999999999999999999999999999999 -->\n", false},
		{"second-line", "\n<!-- archmd template v0.0.9 -->\n", false},
		{"crlf", "<!-- archmd template v0.0.9 | Section layout inspired by https://architecture.md/ -->\r\n", true},
		{"no-newline", "<!-- archmd template v0.0.9 | Section layout inspired by https://architecture.md/ -->", true},
		{"read-error", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := baseProject(t)
			path := filepath.Join(dir, "ARCHITECTURE.md.template")
			if tc.name == "read-error" {
				if err := os.Mkdir(path, 0755); err != nil {
					t.Fatal(err)
				}
			} else if tc.name != "missing" {
				write(t, path, tc.content)
			}
			want := fixture(t, "check-minimal.golden")
			if tc.outdated {
				info := fmt.Sprintf("INFO [outdated-template] ARCHITECTURE.md.template is v0.0.9; this archmd is v%s\n", version)
				want = strings.Replace(want, "lines:", info+"lines:", 1)
				want = strings.Replace(want, "8 INFO", "9 INFO", 1)
			}
			if got := requireSuccess(t, "check", "--dir", dir); got != want {
				t.Fatalf("check:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestCheckOutdatedBlock(t *testing.T) {
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		for _, stored := range []string{"0.0.9", version, "1.0.0", "", "0.x.0", "0.0", "0.0.0.0", "0.0.999999999999999999999999999999"} {
			t.Run(name+"/"+stored, func(t *testing.T) {
				dir := baseProject(t)
				if name == "CLAUDE.md" {
					if err := os.Remove(filepath.Join(dir, "AGENTS.md")); err != nil {
						t.Fatal(err)
					}
				}
				content := "Read ARCHITECTURE.md.\n"
				if stored != "" {
					content += "<!-- archmd:start v" + stored + " -->\r\n<!-- archmd:end -->\r\n"
				}
				write(t, filepath.Join(dir, name), content)
				want := fixture(t, "check-minimal.golden")
				if stored == "0.0.9" {
					info := fmt.Sprintf("INFO [outdated-block] archmd block in %s is v%s; this archmd is v%s\n", name, stored, version)
					want = strings.Replace(want, "lines:", info+"lines:", 1)
					want = strings.Replace(want, "8 INFO", "9 INFO", 1)
				}
				if got := requireSuccess(t, "check", "--dir", dir); got != want {
					t.Fatalf("check:\n%s\nwant:\n%s", got, want)
				}
			})
		}
	}
}

func TestCheckOutdatedBlockPrecedence(t *testing.T) {
	dir := baseProject(t)
	write(t, filepath.Join(dir, "CLAUDE.md"), "ARCHITECTURE.md\n<!-- archmd:start v0.0.1 -->\n")
	for _, marker := range []string{"", "<!-- archmd:start v" + version + " -->\n", "<!-- archmd:start v0.0.9 -->\n"} {
		write(t, filepath.Join(dir, "AGENTS.md"), "ARCHITECTURE.md\n"+marker)
		out := requireSuccess(t, "check", "--dir", dir)
		if strings.Contains(out, "[outdated-block]") != strings.Contains(marker, "v0.0.9") || strings.Contains(out, "block in CLAUDE.md") {
			t.Fatal(out)
		}
	}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	if out := requireSuccess(t, "check", "--dir", dir); strings.Contains(out, "[outdated-") {
		t.Fatal(out)
	}
}

func TestPromptOutdatedDiagnostics(t *testing.T) {
	config(t)
	dir := baseProject(t)
	before := requireSuccess(t, "prompt", "--dir", dir)
	for _, stored := range []string{version, "1.0.0", "invalid"} {
		write(t, filepath.Join(dir, "ARCHITECTURE.md.template"), "<!-- archmd template v"+stored+" -->\n")
		write(t, filepath.Join(dir, "AGENTS.md"), "ARCHITECTURE.md\n<!-- archmd:start v"+stored+" -->\n")
		if got := requireSuccess(t, "prompt", "--dir", dir); got != before {
			t.Fatalf("outdated が無い場合に prompt が変わった:\n%s", got)
		}
	}
	write(t, filepath.Join(dir, "ARCHITECTURE.md.template"), "<!-- archmd template v0.0.9 -->\n")
	write(t, filepath.Join(dir, "AGENTS.md"), "ARCHITECTURE.md\n<!-- archmd:start v0.0.9 -->\n")
	info := fmt.Sprintf("INFO [outdated-template] ARCHITECTURE.md.template is v0.0.9; this archmd is v%s\nINFO [outdated-block] archmd block in AGENTS.md is v0.0.9; this archmd is v%s\n", version, version)
	if got, want := requireSuccess(t, "prompt", "--dir", dir), strings.Replace(before, "lines:", info+"lines:", 1); got != want {
		t.Fatalf("prompt:\n%s\nwant:\n%s", got, want)
	}
}
