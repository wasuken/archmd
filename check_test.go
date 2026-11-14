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
		{"0.2.0", "0.3.1", true},
		{"0.3.2-0.20261114000000-abcdef123456", "0.3.1", false},
		{"0.3.2-0.20261114000000-abcdef123456", "0.3.3", true},
		{"0.3.1", "0.3.2-0.20261114000000-abcdef123456", true},
		{"0.2.0", "dev", false},
		{"dev", "0.3.1", false},
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
		{"0.0.0.0", "0.1.0", true},
		{"0.0.0oops", "0.1.0", true},
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
	setTestVersion(t, "0.3.1")
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
			if tc.name != "missing" {
				warning := "WARN [legacy-template] ARCHITECTURE.md.template is no longer used since v0.2.0. Delete it manually or run: archmd remove --yes\n"
				want = strings.Replace(want, "lines:", warning+"lines:", 1)
				want = strings.Replace(want, "9 WARN", "10 WARN", 1)
			}
			if tc.outdated {
				info := fmt.Sprintf("INFO [outdated-template] ARCHITECTURE.md.template is v0.0.9; this archmd is v%s\n", version)
				want = strings.Replace(want, "lines:", info+"lines:", 1)
				want = strings.Replace(want, "0 INFO", "1 INFO", 1)
			}
			if got := requireSuccess(t, "check", "--dir", dir); got != want {
				t.Fatalf("check:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestCheckOutdatedBlock(t *testing.T) {
	setTestVersion(t, "0.3.1")
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		for _, stored := range []string{"0.0.9", "0.1.0", version, "1.0.0", "", "0.x.0", "0.0", "0.0.999999999999999999999999999999"} {
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
				if stored == "0.0.9" || stored == "0.1.0" {
					info := fmt.Sprintf("INFO [outdated-block] archmd block in %s is v%s; this archmd is v%s\n", name, stored, version)
					want = strings.Replace(want, "lines:", info+"lines:", 1)
					want = strings.Replace(want, "0 INFO", "1 INFO", 1)
				}
				if got := requireSuccess(t, "check", "--dir", dir); got != want {
					t.Fatalf("check:\n%s\nwant:\n%s", got, want)
				}
			})
		}
	}
}

func TestCheckOutdatedBlockPrecedence(t *testing.T) {
	setTestVersion(t, "0.3.1")
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
	setTestVersion(t, "0.3.1")
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

func TestCheckMarkerLineLimits(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		var b strings.Builder
		b.WriteString("# Architecture\n")
		for _, def := range knownSections {
			marker := def.marker()
			if legacy {
				marker = "<!-- arch:" + def.id + " -->"
			}
			b.WriteString(marker + "\n## 見出し\n本文\n")
		}
		dir := baseProject(t)
		write(t, filepath.Join(dir, "ARCHITECTURE.md"), b.String())
		const bodyLines = 25
		for _, limit := range []int{bodyLines, bodyLines - 1} {
			code, out, _ := invoke("check", "--dir", dir, "--max-lines", fmt.Sprint(limit), "--max-section-lines", "2")
			if (code == 1) != (limit < bodyLines) || !strings.Contains(out, "lines: 25\n") || strings.Count(out, "WARN [") != strings.Count(out, "WARN [legacy-format]") || (strings.Count(out, "WARN [legacy-format]") == 1) != legacy {
				t.Fatalf("マーカーを含む行数制限が不正: %d %s", code, out)
			}
		}
		out := requireSuccess(t, "check", "--dir", dir, "--max-section-lines", "1")
		if strings.Count(out, "WARN [section-lines]") != 12 || !strings.Contains(out, "2 lines exceeds limit 1") {
			t.Fatal(out)
		}
	}
}

func TestCheckLegacyFormat(t *testing.T) {
	const warning = "WARN [legacy-format] ARCHITECTURE.md uses the pre-0.2.0 format. Regenerate it: archmd remove --yes && archmd init --write-agents && archmd prompt | <agent>\n"
	for _, oldMarkers := range []int{0, 1, len(knownSections)} {
		text := architectureSkeleton()
		for _, def := range knownSections[:oldMarkers] {
			text = strings.Replace(text, def.marker(), "<!-- arch:"+def.id+" -->", 1)
		}
		dir := baseProject(t)
		write(t, filepath.Join(dir, "ARCHITECTURE.md"), text)
		out := requireSuccess(t, "check", "--dir", dir)
		wantCount := 0
		if oldMarkers > 0 {
			wantCount = 1
		}
		if strings.Count(out, warning) != wantCount || strings.Count(out, "WARN [") != wantCount {
			t.Fatalf("旧形式警告の件数が不正: %s", out)
		}
	}
	// コードフェンス内の例や解析できないマーカーは旧形式検出の対象外。
	dir := baseProject(t)
	write(t, filepath.Join(dir, "ARCHITECTURE.md"), architectureSkeleton()+"```md\n<!-- arch:example -->\n```\n<!-- arch: -->\n")
	if out := requireSuccess(t, "check", "--dir", dir); strings.Contains(out, "[legacy-format]") {
		t.Fatal(out)
	}
}

func TestVersionStampsAndOutdatedDiagnostics(t *testing.T) {
	config(t)
	for _, current := range []string{"0.3.1", "dev", "0.3.3-0.20261114000000-abcdef123456"} {
		t.Run(current, func(t *testing.T) {
			setTestVersion(t, current)
			dir := t.TempDir()
			requireSuccess(t, "init", "--dir", dir)
			if got := requireSuccess(t, "version"); got != current+"\n" {
				t.Fatal(got)
			}
			if got := requireSuccess(t, "template"); !strings.HasPrefix(got, "<!-- archmd template v"+current+" | ") {
				t.Fatal(got)
			}
			if got := read(t, filepath.Join(dir, "AGENTS.md")); !strings.HasPrefix(got, "<!-- archmd:start v"+current+" -->\n") {
				t.Fatal(got)
			}
			for _, stored := range []string{"0.2.0", "0.3.2-0.20261114000000-abcdef123456", "dev"} {
				write(t, filepath.Join(dir, "ARCHITECTURE.md.template"), "<!-- archmd template v"+stored+" | Section layout inspired by https://architecture.md/ -->\n")
				write(t, filepath.Join(dir, "AGENTS.md"), "ARCHITECTURE.md\n<!-- archmd:start v"+stored+" -->\n<!-- archmd:end -->\n")
				out := requireSuccess(t, "check", "--dir", dir)
				want := stored != "dev" && (current == "0.3.1" && stored == "0.2.0" || strings.HasPrefix(current, "0.3.3"))
				for _, code := range []string{"outdated-template", "outdated-block"} {
					if strings.Contains(out, "["+code+"]") != want {
						t.Fatalf("current=%s stored=%s: %s", current, stored, out)
					}
				}
			}
		})
	}
}
