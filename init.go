package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func initProject(dir string, maxLines int, out, errOut io.Writer) (resultErr error) {
	commonPath, found, err := commonRulesLocation()
	if err != nil {
		return err
	}
	if found {
		_, err = fmt.Fprintf(errOut, "Using common rules from %s\n", commonPath)
	} else {
		_, err = fmt.Fprintf(errOut, "Tip: shared rules (e.g. document language) can go in %s\n", commonPath)
	}
	if err != nil {
		return err
	}
	created := false
	defer func() {
		if created && resultErr == nil {
			_, resultErr = io.WriteString(errOut, nextGuidance)
		}
	}()
	for _, file := range []struct{ name, content string }{
		{"ARCHITECTURE.md.template", architectureTemplate},
		{"ARCHITECTURE.md", architectureSkeleton()},
	} {
		path := filepath.Join(dir, file.name)
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(errOut, "%s: skipped (exists)\n", file.name)
		} else if !os.IsNotExist(err) {
			return err
		} else {
			if err := os.WriteFile(path, []byte(file.content), 0644); err != nil {
				return err
			}
			if file.name == "ARCHITECTURE.md" {
				created = true
			}
			fmt.Fprintf(errOut, "%s: created\n", file.name)
		}
	}
	files, err := agentFiles(dir)
	if err != nil {
		return err
	}
	for _, content := range files {
		if strings.Contains(string(content), "<!-- archmd:start") {
			fmt.Fprintln(errOut, "archmd block already exists")
			return nil
		}
	}
	block, err := agentsBlock(maxLines)
	if err != nil {
		return err
	}
	if len(files) > 0 {
		if _, err := io.WriteString(out, block); err != nil {
			return err
		}
		fmt.Fprintln(errOut, "Append this block manually to your existing AGENTS.md or CLAUDE.md.")
		return nil
	}
	return os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(block), 0644)
}
