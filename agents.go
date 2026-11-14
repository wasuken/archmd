package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// rawLines retains every byte, including each line's original newline sequence.
func rawLines(data []byte) [][]byte {
	var lines [][]byte
	for start := 0; start < len(data); {
		end := start
		for end < len(data) && data[end] != '\r' && data[end] != '\n' {
			end++
		}
		if end < len(data) {
			if data[end] == '\r' && end+1 < len(data) && data[end+1] == '\n' {
				end++
			}
			end++
		}
		lines = append(lines, data[start:end])
		start = end
	}
	return lines
}

func stripArchmdBlocks(data []byte) ([]byte, bool, error) {
	var result bytes.Buffer
	inside, found, skipBlank := false, false, false
	for _, line := range rawLines(data) {
		starts := bytes.Count(line, []byte("<!-- archmd:start"))
		ends := bytes.Count(line, []byte("<!-- archmd:end -->"))
		if starts > 1 || ends > 1 || starts > 0 && inside {
			return nil, false, fmt.Errorf("remove: malformed or nested archmd markers")
		}
		if starts > 0 {
			if ends > 0 && bytes.Index(line, []byte("<!-- archmd:end -->")) < bytes.Index(line, []byte("<!-- archmd:start")) {
				return nil, false, fmt.Errorf("remove: archmd end marker precedes start marker")
			}
			inside, found = true, true
		}
		if ends > 0 {
			if !inside {
				return nil, false, fmt.Errorf("remove: archmd end marker has no preceding start marker")
			}
			inside, skipBlank = false, true
			continue
		}
		if inside {
			continue
		}
		if skipBlank {
			skipBlank = false
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
		}
		result.Write(line)
	}
	if inside {
		return nil, false, fmt.Errorf("remove: archmd start marker has no end marker")
	}
	return result.Bytes(), found, nil
}

func agentsBlock(maxLines int) (string, error) {
	rules, err := agentsRules(maxLines)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("<!-- archmd:start v%s -->\n%s<!-- archmd:end -->\n", version, rules), nil
}

func commonRulesPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "architecture-md", "common.md"), nil
}

func commonRulesLocation() (string, bool, error) {
	path, err := commonRulesPath()
	if err != nil {
		return "", false, err
	}
	_, err = os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return "", false, err
	}
	return path, err == nil, nil
}

func agentsRules(maxLines int) (string, error) {
	path, err := commonRulesPath()
	if err != nil {
		return "", err
	}
	common, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(normalize(agentsTemplate), "\n") + "\n")
	if len(common) > 0 {
		b.WriteString(strings.TrimRight(normalize(string(common)), "\n") + "\n")
	}
	if maxLines > 0 {
		fmt.Fprintf(&b, "- Keep ARCHITECTURE.md within %d lines; revise it until `archmd check --max-lines %d` passes.\n", maxLines, maxLines)
		b.WriteString("- Compress in this order: detailed schemas, Glossary, Roadmap, then long setup instructions (move these to CONTRIBUTING.md). Preserve Project layout, System map, and Components.\n")
	}
	return b.String(), nil
}

func agentFiles(dir string) ([][]byte, error) {
	var files [][]byte
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		files = append(files, content)
	}
	return files, nil
}
func hasArchitectureReference(content []byte) bool {
	return strings.Contains(string(content), "ARCHITECTURE.md")
}

func preferredAgentInstructions(dir string) ([]byte, string, error) {
	content, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if os.IsNotExist(err) {
		content, err = os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
		return content, "CLAUDE.md", err
	}
	return content, "AGENTS.md", err
}
