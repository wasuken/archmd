package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInitWriteAgentsAppendAndRemove(t *testing.T) {
	for _, tc := range []struct {
		name, original, separator string
	}{
		{"empty", "", ""},
		{"no-newline", "既存の指示\x00", "\n\n"},
		{"lf", "既存の指示\n", "\n"},
		{"crlf", "既存の指示\r\n", "\n"},
		{"cr", "既存の指示\r", "\r"},
		{"blank-line", "既存の指示\n\n", ""},
		{"blank-crlf", "既存の指示\r\n\r\n", ""},
		{"whitespace-line", "既存の指示\n \t\n", ""},
		{"unterminated-blank", "既存の指示\n \t", "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config(t)
			dir := t.TempDir()
			path := filepath.Join(dir, "AGENTS.md")
			write(t, path, tc.original)
			code, out, stderr := invoke("init", "--write-agents", "--dir", dir)
			if code != 0 || out != "" || !strings.Contains(stderr, "appended archmd block to "+path+"\n") {
				t.Fatalf("init: %d %q %q", code, out, stderr)
			}
			got := read(t, path)
			if !strings.HasPrefix(got, tc.original) || got != tc.original+tc.separator+fixture(t, "agents.golden") {
				t.Fatalf("既存バイト列または区切りが不正: %q", got)
			}
			before := snapshotPromptFiles(t, dir)
			code, out, stderr = invoke("init", "--write-agents", "--dir", dir)
			if code != 0 || out != "" || !strings.Contains(stderr, "skipped: "+path+" already contains the archmd block\n") {
				t.Fatalf("second init: %d %q %q", code, out, stderr)
			}
			if !reflect.DeepEqual(before, snapshotPromptFiles(t, dir)) {
				t.Fatal("再実行でファイルが変わった")
			}
			requireSuccess(t, "remove", "--yes", "--dir", dir)
			data, err := os.ReadFile(path)
			if os.IsNotExist(err) && strings.TrimSpace(tc.original) == "" {
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimRight(string(data), " \t\r\n") != strings.TrimRight(tc.original, " \t\r\n") {
				t.Fatalf("remove で元の内容に戻らない: %q", data)
			}
		})
	}
}

func TestInitWriteAgentsSelection(t *testing.T) {
	for _, names := range [][]string{{"CLAUDE.md"}, {"AGENTS.md", "CLAUDE.md"}} {
		t.Run(strings.Join(names, "+"), func(t *testing.T) {
			config(t)
			dir := t.TempDir()
			for _, name := range names {
				// 優先対象外のブロックは追記の判定に影響しない。
				content := "original\r\n"
				if len(names) > 1 && name == "CLAUDE.md" {
					content += fixture(t, "agents.golden")
				}
				write(t, filepath.Join(dir, name), content)
			}
			originalClaude := read(t, filepath.Join(dir, "CLAUDE.md"))
			code, out, stderr := invoke("init", "--write-agents", "--dir", dir)
			path := filepath.Join(dir, names[0])
			if code != 0 || out != "" || !strings.Contains(stderr, "appended archmd block to "+path+"\n") {
				t.Fatalf("init: %d %q %q", code, out, stderr)
			}
			if read(t, path) != "original\r\n\n"+fixture(t, "agents.golden") {
				t.Fatal("対象ファイルへの追記が不正")
			}
			if len(names) == 1 {
				if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); !os.IsNotExist(err) {
					t.Fatal("AGENTS.md が作られた")
				}
			} else if read(t, filepath.Join(dir, "CLAUDE.md")) != originalClaude {
				t.Fatal("CLAUDE.md が変更された")
			}
		})
	}
}

func TestInitWriteAgentsNewFileMatchesDefault(t *testing.T) {
	config(t)
	plain, flagged := t.TempDir(), t.TempDir()
	code, out, stderr := invoke("init", "--dir", plain)
	flagCode, flagOut, flagErr := invoke("init", "--write-agents", "--dir", flagged)
	if code != 0 || flagCode != code || flagOut != out || flagErr != stderr {
		t.Fatalf("新規作成時の出力が異なる: %d %q %q / %d %q %q", code, out, stderr, flagCode, flagOut, flagErr)
	}
	for _, name := range []string{"ARCHITECTURE.md", "ARCHITECTURE.md.template", "AGENTS.md"} {
		if read(t, filepath.Join(plain, name)) != read(t, filepath.Join(flagged, name)) {
			t.Fatalf("新規作成時のバイト列が異なる: %s", name)
		}
	}
}

func TestInitWriteAgentsMalformedMarkers(t *testing.T) {
	for _, content := range []string{
		"before\n<!-- archmd:start v0.1.0 -->\n",
		"before\n<!-- archmd:end -->\n",
		"<!-- archmd:end -->\n<!-- archmd:start v0.1.0 -->\n",
		"<!-- archmd:end --><!-- archmd:start v0.1.0 -->\n",
		"<!-- archmd:start --><!-- archmd:start --><!-- archmd:end -->\n",
	} {
		t.Run(content, func(t *testing.T) {
			config(t)
			dir := t.TempDir()
			write(t, filepath.Join(dir, "AGENTS.md"), content)
			before := snapshotPromptFiles(t, dir)
			code, out, stderr := invoke("init", "--write-agents", "--dir", dir)
			if code != 1 || out != "" || stderr == "" {
				t.Fatalf("不正マーカー: %d %q %q", code, out, stderr)
			}
			if !reflect.DeepEqual(before, snapshotPromptFiles(t, dir)) {
				t.Fatal("不正マーカーでファイルが変更された")
			}
		})
	}
}

func TestInitWriteAgentsCommonAndMaxLines(t *testing.T) {
	conf := config(t)
	common := filepath.Join(conf, "architecture-md", "common.md")
	if err := os.MkdirAll(filepath.Dir(common), 0755); err != nil {
		t.Fatal(err)
	}
	write(t, common, "- 文書は日本語で書く。\r\n")
	plain, flagged := t.TempDir(), t.TempDir()
	for _, dir := range []string{plain, flagged} {
		write(t, filepath.Join(dir, "AGENTS.md"), "original")
	}
	block := requireSuccess(t, "init", "--dir", plain, "--max-lines", "300")
	if !strings.Contains(block, "文書は日本語で書く。\n") || !strings.Contains(block, "within 300 lines") || !strings.Contains(block, "archmd check --max-lines 300") {
		t.Fatal("共通ルールまたは行数制限がない")
	}
	if out := requireSuccess(t, "init", "--dir", flagged, "--write-agents", "--max-lines", "300"); out != "" {
		t.Fatalf("追記時の標準出力: %q", out)
	}
	if got := read(t, filepath.Join(flagged, "AGENTS.md")); got != "original\n\n"+block {
		t.Fatalf("通常 init のブロックと異なる: %q", got)
	}
}

func TestInitWriteAgentsHelp(t *testing.T) {
	config(t)
	for _, args := range [][]string{{"help"}, {"--help"}, {"init", "--help"}} {
		code, out, stderr := invoke(args...)
		if code != 0 || !strings.Contains(out+stderr, writeAgentsHelp) {
			t.Fatalf("%v: %d %q %q", args, code, out, stderr)
		}
	}
	code, _, _ := invoke("check", "--write-agents")
	if code != 2 {
		t.Fatal("init 以外で --write-agents が受理された")
	}
}
