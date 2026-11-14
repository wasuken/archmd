package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateVersionAndSkeletonHeader(t *testing.T) {
	config(t)
	dir := t.TempDir()
	requireSuccess(t, "init", "--dir", dir)
	output := requireSuccess(t, "template")
	installed := read(t, filepath.Join(dir, "ARCHITECTURE.md.template"))
	header := "<!-- archmd template v" + version + " | Section layout inspired by https://architecture.md/ -->"
	for _, content := range []string{output, installed} {
		first, _, _ := strings.Cut(content, "\n")
		if first != header || strings.Contains(content, "{{VERSION}}") {
			t.Fatalf("テンプレートの版が不正: %q", first)
		}
		if content != strings.Replace(architectureTemplateSource, "{{VERSION}}", version, 1) {
			t.Fatal("プレースホルダー以外の本文が変わった")
		}
	}
	first, _, _ := strings.Cut(read(t, filepath.Join(dir, "ARCHITECTURE.md")), "\n")
	if first != "<!-- Section layout inspired by https://architecture.md/ -->" {
		t.Fatalf("骨組みの先頭コメントが変わった: %q", first)
	}
}

func TestInitPreservesExistingTemplate(t *testing.T) {
	config(t)
	for _, original := range []string{
		"<!-- Section layout inspired by https://architecture.md/ -->\r\n手書きの内容",
		"<!-- archmd template v0.0.9 -->\n手書きの内容\n",
	} {
		dir := t.TempDir()
		path := filepath.Join(dir, "ARCHITECTURE.md.template")
		write(t, path, original)
		requireSuccess(t, "init", "--dir", dir)
		if got := read(t, path); got != original {
			t.Fatalf("既存テンプレートが上書きされた: %q", got)
		}
	}
}
