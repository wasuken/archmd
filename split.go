package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func splitProject(dir string) error {
	target := filepath.Join(dir, ".architecture")
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("split: .architecture already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	data, err := os.ReadFile(filepath.Join(dir, "ARCHITECTURE.md"))
	if err != nil {
		return err
	}
	d := parseDocument(string(data))
	lines := rawLines(data)
	// マーカーのない H2 だけ分割を止め、不正な本文は check の警告に任せる。
	for _, line := range d.unknownHeadings {
		if line <= len(d.lines) && isH2(d.lines[line-1]) {
			return fmt.Errorf("split: H2 at line %d has no marker; add an ID marker first", line)
		}
	}
	seen := map[string]bool{}
	for _, s := range d.sections {
		if seen[s.id] {
			return fmt.Errorf("split: duplicate ID %q", s.id)
		}
		seen[s.id] = true
		for _, c := range s.id {
			if c < 32 || strings.ContainsRune(`<>:"/\|?*`, c) {
				return fmt.Errorf("split: ID %q cannot be used safely in a filename", s.id)
			}
		}
	}
	if err := os.Mkdir(target, 0755); err != nil {
		return err
	}
	headerEnd := len(d.lines)
	if len(d.sections) > 0 {
		headerEnd = d.sections[0].start
	}
	writePart := func(name string, lines [][]byte) error {
		return os.WriteFile(filepath.Join(target, name), bytes.Join(lines, nil), 0644)
	}
	if err := writePart("00-header.md", lines[:headerEnd]); err != nil {
		return err
	}
	for i, s := range d.sections {
		if err := writePart(fmt.Sprintf("%02d-%s.md", i+1, s.id), lines[s.start:s.end]); err != nil {
			return err
		}
	}
	if err := buildProject(dir); err != nil {
		return err
	}
	actual, err := os.ReadFile(filepath.Join(dir, "ARCHITECTURE.md"))
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, data) {
		return fmt.Errorf("split: drift after build")
	}
	return nil
}
