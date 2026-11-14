package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func invoke(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func fixture(t *testing.T, name string) string {
	t.Helper()
	return read(t, filepath.Join("testdata", name))
}
func config(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("AppData", d)
	case "darwin", "ios":
		t.Setenv("HOME", d)
	default:
		t.Setenv("XDG_CONFIG_HOME", d)
	}
	actual, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	return actual
}
func requireSuccess(t *testing.T, args ...string) string {
	t.Helper()
	code, out, stderr := invoke(args...)
	if code != 0 {
		t.Fatalf("%v: code %d\n%s\n%s", args, code, out, stderr)
	}
	return out
}
func baseProject(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	write(t, filepath.Join(d, "ARCHITECTURE.md"), fixture(t, "minimal.md"))
	write(t, filepath.Join(d, "AGENTS.md"), "Read ARCHITECTURE.md.\n")
	return d
}

func TestInitAndIdempotence(t *testing.T) {
	config(t)
	dir := t.TempDir()
	requireSuccess(t, "init", "--dir", dir)
	archPath, agentsPath := filepath.Join(dir, "ARCHITECTURE.md"), filepath.Join(dir, "AGENTS.md")
	arch, agents := read(t, archPath), read(t, agentsPath)
	if arch != architectureSkeleton() || agents != fixture(t, "agents.golden") {
		t.Fatal("init content differs from template or agents golden")
	}
	archInfo, _ := os.Stat(archPath)
	agentsInfo, _ := os.Stat(agentsPath)
	code, out, stderr := invoke("init", "--dir", dir)
	if code != 0 || out != "" || !strings.Contains(stderr, "skipped (exists)") || !strings.Contains(stderr, "block already exists") {
		t.Fatalf("second init: %d %q %q", code, out, stderr)
	}
	if read(t, archPath) != arch || read(t, agentsPath) != agents {
		t.Fatal("second init changed bytes")
	}
	afterArch, _ := os.Stat(archPath)
	afterAgents, _ := os.Stat(agentsPath)
	if !archInfo.ModTime().Equal(afterArch.ModTime()) || !agentsInfo.ModTime().Equal(afterAgents.ModTime()) {
		t.Fatal("second init rewrote files")
	}
	if out := requireSuccess(t, "check", "--dir", dir); !strings.Contains(out, "summary: 0 ERROR, 0 WARN, 0 INFO") {
		t.Fatal(out)
	}
}

func TestInitExistingInstructionsAndCommon(t *testing.T) {
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		for _, withCommon := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/common=%v", name, withCommon), func(t *testing.T) {
				conf := config(t)
				if withCommon {
					folder := filepath.Join(conf, "architecture-md")
					if err := os.MkdirAll(folder, 0755); err != nil {
						t.Fatal(err)
					}
					write(t, filepath.Join(folder, "common.md"), "- Shared custom rules.\r\n")
				}
				dir := t.TempDir()
				original := "custom instructions\r\n\x00"
				path := filepath.Join(dir, name)
				write(t, path, original)
				code, out, stderr := invoke("init", "--dir", dir)
				if code != 0 || read(t, path) != original || !strings.Contains(out, "<!-- archmd:start v"+version+" -->") || !strings.Contains(out, "<!-- archmd:end -->") || !strings.Contains(stderr, "manually") {
					t.Fatalf("init: %d %q %q", code, out, stderr)
				}
				if strings.Contains(out, "Shared custom rules.") != withCommon || strings.Contains(out, "\r") {
					t.Fatal("common block incorrect")
				}
				if name == "CLAUDE.md" {
					if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); !os.IsNotExist(err) {
						t.Fatal("AGENTS.md created")
					}
				}
			})
		}
	}
}

func TestInitNewCommonAndMaxLines(t *testing.T) {
	conf := config(t)
	folder := filepath.Join(conf, "architecture-md")
	if err := os.MkdirAll(folder, 0755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(folder, "common.md"), "- Shared rules.\r\n")
	dir := t.TempDir()
	requireSuccess(t, "init", "--dir", dir, "--max-lines", "300")
	block := read(t, filepath.Join(dir, "AGENTS.md"))
	for _, want := range []string{"Shared rules.", "within 300 lines", "archmd check --max-lines 300", "detailed schemas, Glossary, Roadmap, then long setup instructions", "CONTRIBUTING.md", "Preserve Project layout, System map, and Components"} {
		if !strings.Contains(block, want) {
			t.Errorf("block missing %q", want)
		}
	}
	if strings.Contains(block, "\r") {
		t.Fatal("CR in generated block")
	}
}

func TestInitRecognizesBlockInEitherFile(t *testing.T) {
	config(t)
	dir := t.TempDir()
	write(t, filepath.Join(dir, "AGENTS.md"), "local rules\r\n")
	write(t, filepath.Join(dir, "CLAUDE.md"), "<!-- archmd:start custom -->\r\n")
	code, out, stderr := invoke("init", "--dir", dir)
	if code != 0 || out != "" || !strings.Contains(stderr, "block already exists") {
		t.Fatalf("%d %q %q", code, out, stderr)
	}
	if read(t, filepath.Join(dir, "AGENTS.md")) != "local rules\r\n" || read(t, filepath.Join(dir, "CLAUDE.md")) != "<!-- archmd:start custom -->\r\n" {
		t.Fatal("instructions changed")
	}
}

func TestCheckGolden(t *testing.T) {
	dir := baseProject(t)
	out := requireSuccess(t, "check", "--dir", dir)
	if out != fixture(t, "check-minimal.golden") {
		t.Fatalf("check golden mismatch:\n%s", out)
	}
}

func TestCheckDiagnosticCodes(t *testing.T) {
	tests := []struct {
		name, text, agent, code string
		exit                    int
		args                    []string
	}{
		{name: "required", text: "# Empty\n", code: "required-missing", exit: 1},
		{name: "duplicate", text: "<!-- arch:project-structure -->\n## Again\n", code: "duplicate-id", exit: 1},
		{name: "max", code: "max-lines", exit: 1, args: []string{"--max-lines", "10"}},
		{name: "heading", text: "## Unmarked\n", code: "unknown-heading"},
		{name: "malformed-heading", text: "<!-- arch:custom -->\ntext\n", code: "unknown-heading"},
		{name: "unknown", text: "<!-- arch:custom -->\n## Custom\n", code: "unknown-id"},
		{name: "section", code: "section-lines", args: []string{"--max-section-lines", "2"}},
		{name: "reference", agent: "no mention", code: "no-reference"},
		{name: "agents", agent: "REMOVE", code: "no-agents-file"},
		{name: "missing-file", text: "REMOVE", code: "no-architecture-file", exit: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := baseProject(t)
			path := filepath.Join(dir, "ARCHITECTURE.md")
			if tc.name == "required" {
				write(t, path, tc.text)
			} else if tc.text == "REMOVE" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else if tc.text != "" {
				write(t, path, read(t, path)+tc.text)
			}
			if tc.agent == "REMOVE" {
				if err := os.Remove(filepath.Join(dir, "AGENTS.md")); err != nil {
					t.Fatal(err)
				}
			} else if tc.agent != "" {
				write(t, filepath.Join(dir, "AGENTS.md"), tc.agent)
			}
			args := append([]string{"check", "--dir", dir}, tc.args...)
			code, out, stderr := invoke(args...)
			if code != tc.exit || !strings.Contains(out, "["+tc.code+"]") {
				t.Fatalf("%d %s %s", code, out, stderr)
			}
			if tc.name == "max" && !strings.Contains(out, "current 13 lines, limit 10, excess 3") {
				t.Fatal(out)
			}
			if !strings.Contains(out, "lines: ") || !strings.Contains(out, "summary: ") {
				t.Fatal("missing totals")
			}
		})
	}
}

func TestCheckMax300(t *testing.T) {
	for _, n := range []int{299, 300, 301} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			dir := baseProject(t)
			text := fixture(t, "minimal.md")
			text += strings.Repeat("body\n", n-len(linesOf(text))+len(parseDocument(text).sections))
			write(t, filepath.Join(dir, "ARCHITECTURE.md"), text)
			code, out, _ := invoke("check", "--dir", dir, "--max-lines", "300")
			want := 0
			if n > 300 {
				want = 1
			}
			if code != want || !strings.Contains(out, fmt.Sprintf("lines: %d\n", n)) {
				t.Fatal(code, out)
			}
		})
	}
}

func TestSplitBuildRoundTripAndDrift(t *testing.T) {
	dir := baseProject(t)
	requireSuccess(t, "split", "--dir", dir)
	expectedNames := []string{"00-header.md", "01-project-structure.md", "02-high-level-diagram.md", "03-core-components.md", "04-project-identification.md"}
	entries, err := os.ReadDir(filepath.Join(dir, ".architecture"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(expectedNames) {
		t.Fatal(entries)
	}
	for i, entry := range entries {
		if entry.Name() != expectedNames[i] {
			t.Fatal(entry.Name())
		}
	}
	path := filepath.Join(dir, "ARCHITECTURE.md")
	first := read(t, path)
	if first != fixture(t, "build.golden") {
		t.Fatalf("build mismatch:\n%s", first)
	}
	requireSuccess(t, "check", "--dir", dir)
	requireSuccess(t, "build", "--dir", dir)
	requireSuccess(t, "build", "--dir", dir)
	if first != read(t, path) {
		t.Fatal("build not deterministic")
	}
	write(t, path, first+"manual edit\n")
	code, out, _ := invoke("check", "--dir", dir)
	if code != 1 || !strings.Contains(out, "ERROR [drift]") {
		t.Fatal(code, out)
	}
	requireSuccess(t, "build", "--dir", dir)
	write(t, path, strings.ReplaceAll(read(t, path), "\n", "\r\n"))
	code, out, _ = invoke("check", "--dir", dir)
	if code != 1 || !strings.Contains(out, "ERROR [drift]") {
		t.Fatal("drift must compare raw bytes", out)
	}
}

func TestSplitPreflightAndGeneratedHeader(t *testing.T) {
	for _, suffix := range []string{"## Unmarked\n", "<!-- arch:../unsafe -->\n## Unsafe\n", "<!-- arch:core-components -->\n## Duplicate\n"} {
		dir := baseProject(t)
		path := filepath.Join(dir, "ARCHITECTURE.md")
		original := read(t, path) + suffix
		write(t, path, original)
		code, _, _ := invoke("split", "--dir", dir)
		if code != 1 || read(t, path) != original {
			t.Fatal("split should fail without changing input")
		}
		if _, err := os.Stat(filepath.Join(dir, ".architecture")); !os.IsNotExist(err) {
			t.Fatal("split wrote before validation")
		}
	}
	dir := baseProject(t)
	path := filepath.Join(dir, "ARCHITECTURE.md")
	write(t, path, generatedHeader+"\r\n\r\n"+strings.ReplaceAll(read(t, path), "\n", "\r\n"))
	requireSuccess(t, "split", "--dir", dir)
	if strings.Count(read(t, path), generatedHeader) != 1 || !strings.Contains(read(t, path), "\r") {
		t.Fatal("generated header or original newline changed")
	}
	before := read(t, path)
	code, _, _ := invoke("split", "--dir", dir)
	if code != 1 || read(t, path) != before {
		t.Fatal("existing split overwritten")
	}
}

func TestBuildOrderAndNormalization(t *testing.T) {
	dir := t.TempDir()
	code, _, _ := invoke("build", "--dir", dir)
	if code != 1 {
		t.Fatal("missing directory accepted")
	}
	source := filepath.Join(dir, ".architecture")
	if err := os.Mkdir(source, 0755); err != nil {
		t.Fatal(err)
	}
	requireSuccess(t, "build", "--dir", dir)
	if got := read(t, filepath.Join(dir, "ARCHITECTURE.md")); got != generatedHeader+"\n" {
		t.Fatal(got)
	}
	write(t, filepath.Join(source, "20-b.md"), "B\r\n\r\n")
	write(t, filepath.Join(source, "10-a.md"), "A")
	write(t, filepath.Join(source, "ignore.txt"), "ignore")
	if err := os.Mkdir(filepath.Join(source, "30-directory.md"), 0755); err != nil {
		t.Fatal(err)
	}
	requireSuccess(t, "build", "--dir", dir)
	if got := read(t, filepath.Join(dir, "ARCHITECTURE.md")); got != generatedHeader+"\n\nA\n\nB\n" {
		t.Fatal(got)
	}
}

func TestAgentReferencePrecedence(t *testing.T) {
	dir := baseProject(t)
	write(t, filepath.Join(dir, "CLAUDE.md"), "No reference.")
	out := requireSuccess(t, "check", "--dir", dir)
	if strings.Contains(out, "[no-reference]") {
		t.Fatal(out)
	}
	write(t, filepath.Join(dir, "AGENTS.md"), "No reference.")
	write(t, filepath.Join(dir, "CLAUDE.md"), "ARCHITECTURE.md")
	out = requireSuccess(t, "check", "--dir", dir)
	if !strings.Contains(out, "[no-reference]") {
		t.Fatal(out)
	}
	if err := os.Remove(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	out = requireSuccess(t, "check", "--dir", dir)
	if strings.Contains(out, "[no-reference]") {
		t.Fatal(out)
	}
}

func TestCLIArgumentsAndOutputs(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"check", "--bogus"}, {"check", "--max-lines", "0"}, {"init", "--max-lines", "-1"}, {"check", "--max-section-lines", "0"}, {"check", "--max-lines", "oops"}, {"template", "extra"}} {
		code, _, _ := invoke(args...)
		if code != 2 {
			t.Errorf("%v: got %d", args, code)
		}
	}
	for _, tc := range []struct{ cmd, expected string }{{"template", architectureTemplate}, {"notices", thirdPartyNotices}, {"version", version + "\n"}} {
		if got := requireSuccess(t, tc.cmd, "--dir", t.TempDir()); got != tc.expected {
			t.Errorf("%s differs", tc.cmd)
		}
	}
	dir := baseProject(t)
	requireSuccess(t, "--dir", dir, "check")

}

func TestQualityChecks(t *testing.T) {
	for _, command := range [][]string{{"go", "vet", "./..."}, {"gofmt", "-l", "."}} {
		out, err := exec.Command(command[0], command[1:]...).CombinedOutput()
		if err != nil || len(out) != 0 {
			t.Errorf("%v: %v\n%s", command, err, out)
		}
	}
}

func TestCheckIgnoresUnusedClaudeReadError(t *testing.T) {
	dir := baseProject(t)
	if err := os.Mkdir(filepath.Join(dir, "CLAUDE.md"), 0755); err != nil {
		t.Fatal(err)
	}
	requireSuccess(t, "check", "--dir", dir)
}

func TestSplitEmptyHeaderAndCustomID(t *testing.T) {
	for _, text := range []string{"", "<!-- arch:custom-日本語 -->\n## Custom\n"} {
		dir := t.TempDir()
		write(t, filepath.Join(dir, "ARCHITECTURE.md"), text)
		requireSuccess(t, "split", "--dir", dir)
		if read(t, filepath.Join(dir, ".architecture", "00-header.md")) != "" {
			t.Fatal("empty header gained content")
		}
		if text != "" && read(t, filepath.Join(dir, ".architecture", "01-custom-日本語.md")) != text {
			t.Fatal("custom ID was not preserved")
		}
	}
}

func TestSplitBuildEmptySectionsPreservesBytes(t *testing.T) {
	config(t)
	for _, newline := range []string{"\n", "\r\n", "\r"} {
		for _, finalNewline := range []bool{false, true} {
			dir := t.TempDir()
			requireSuccess(t, "init", "--dir", dir)
			path := filepath.Join(dir, "ARCHITECTURE.md")
			original := strings.TrimSuffix(read(t, path), "\n")
			if finalNewline {
				original += "\n"
			}
			original = strings.ReplaceAll(original, "\n", newline)
			write(t, path, original)
			requireSuccess(t, "split", "--dir", dir)
			for i, def := range knownSections {
				if def.required {
					continue
				}
				part := read(t, filepath.Join(dir, ".architecture", fmt.Sprintf("%02d-%s.md", i+1, def.id)))
				want := def.marker() + newline
				if i == len(knownSections)-1 && !finalNewline {
					want = def.marker()
				}
				if part != want {
					t.Fatalf("空の節 %s の内容が不正: %q", def.id, part)
				}
			}
			requireSuccess(t, "build", "--dir", dir)
			if read(t, path) != original {
				t.Fatal("split → build でバイト列が変わった")
			}
			if out := requireSuccess(t, "check", "--dir", dir); !strings.Contains(out, "summary: 0 ERROR, 0 WARN, 0 INFO") {
				t.Fatal(out)
			}
		}
	}
}

func TestCommonReadError(t *testing.T) {
	conf := config(t)
	if err := os.MkdirAll(filepath.Join(conf, "architecture-md", "common.md"), 0755); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	code, _, _ := invoke("init", "--dir", dir)
	if code != 1 {
		t.Fatal("common.md read error ignored")
	}
}
