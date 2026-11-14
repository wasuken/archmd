package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func initProject(dir string, maxLines int, writeAgents bool, out, errOut io.Writer) (resultErr error) {
	var agentPath string
	var agentContent []byte
	var hasBlock bool
	if writeAgents {
		for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
			path := filepath.Join(dir, name)
			content, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			_, found, err := stripArchmdBlocks(content)
			if err != nil {
				return fmt.Errorf("init: %s: %w", path, err)
			}
			agentPath, agentContent, hasBlock = path, content, found
			break
		}
	}
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
	if writeAgents && agentPath != "" {
		if hasBlock {
			_, err := fmt.Fprintf(errOut, "skipped: %s already contains the archmd block\n", agentPath)
			return err
		}
		block, err := agentsBlock(maxLines)
		if err != nil {
			return err
		}
		if err := appendAgentsBlock(agentPath, agentContent, block); err != nil {
			return err
		}
		_, err = fmt.Fprintf(errOut, "appended archmd block to %s\n", agentPath)
		return err
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

func appendAgentsBlock(path string, content []byte, block string) error {
	separator := ""
	lines := rawLines(content)
	if len(lines) > 0 {
		last := string(lines[len(lines)-1])
		if !strings.HasSuffix(last, "\n") && !strings.HasSuffix(last, "\r") {
			separator = "\n"
		}
		if strings.TrimSpace(last) != "" {
			if strings.HasSuffix(last, "\r") {
				separator += "\r"
			} else {
				separator += "\n"
			}
		}
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	_, writeErr := io.WriteString(f, separator+block)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
