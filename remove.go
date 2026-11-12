package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type removalTarget struct {
	name, action, result string
	content              []byte
}

func removalInfo(path string, directory bool) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if directory && !info.IsDir() || !directory && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("remove: %s has an unsupported file type (symlinks are not followed)", path)
	}
	return info, nil
}

func removalPlan(dir string, keepArchitecture bool) ([]removalTarget, error) {
	architecture := removalTarget{name: "ARCHITECTURE.md", result: "skipped(not found)"}
	info, err := removalInfo(filepath.Join(dir, architecture.name), false)
	if err != nil {
		return nil, err
	}
	if info != nil {
		architecture.action, architecture.result = "remove", "removed"
		if keepArchitecture {
			architecture.action, architecture.result = "", "kept"
			data, err := os.ReadFile(filepath.Join(dir, architecture.name))
			if err != nil {
				return nil, err
			}
			lines := rawLines(data)
			if len(lines) > 0 && string(bytes.TrimRight(lines[0], "\r\n")) == generatedHeader {
				lines = lines[1:]
				if len(lines) > 0 && len(bytes.TrimSpace(lines[0])) == 0 {
					lines = lines[1:]
				}
				architecture.action, architecture.content = "write", bytes.Join(lines, nil)
			}
		}
	}
	fragments := removalTarget{name: ".architecture", result: "skipped(not found)"}
	info, err = removalInfo(filepath.Join(dir, fragments.name), true)
	if err != nil {
		return nil, err
	}
	if info != nil {
		fragments.action, fragments.result = "remove-all", "removed"
	}
	instructions := removalTarget{name: "AGENTS.md", result: "skipped(not found)"}
	info, err = removalInfo(filepath.Join(dir, instructions.name), false)
	if err != nil {
		return nil, err
	}
	if info == nil {
		instructions.name = "CLAUDE.md"
		info, err = removalInfo(filepath.Join(dir, instructions.name), false)
		if err != nil {
			return nil, err
		}
	}
	if info != nil {
		data, err := os.ReadFile(filepath.Join(dir, instructions.name))
		if err != nil {
			return nil, err
		}
		content, found, err := stripArchmdBlocks(data)
		if err != nil {
			return nil, err
		}
		instructions.result = "kept (no archmd block)"
		if found {
			instructions.action, instructions.result, instructions.content = "write", "removed (archmd block)", content
			if len(bytes.TrimSpace(content)) == 0 {
				instructions.action, instructions.result = "remove", "removed (file contained only archmd blocks and whitespace)"
			}
		}
	}
	return []removalTarget{architecture, fragments, instructions}, nil
}

func removeProject(dir string, yes, keepArchitecture bool, out io.Writer) error {
	warnings := "WARNING: マーカー間の内容は、ユーザーが手で書き足した部分も含めて消えます。\n" +
		"WARNING: ARCHITECTURE.md と .architecture/ は再生成できません。\n"
	gitInfo, err := os.Stat(filepath.Join(dir, ".git"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if gitInfo == nil || !gitInfo.IsDir() {
		warnings += "WARNING: .git ディレクトリが無く、git管理外のため復元できない。\n"
	}
	if _, err := io.WriteString(out, warnings); err != nil {
		return err
	}
	targets, err := removalPlan(dir, keepArchitecture)
	if err != nil {
		return err
	}
	for _, target := range targets {
		result := target.result
		if yes {
			path := filepath.Join(dir, target.name)
			switch target.action {
			case "remove":
				err = os.Remove(path)
			case "remove-all":
				err = os.RemoveAll(path)
			case "write":
				err = os.WriteFile(path, target.content, 0644)
			}
			if err != nil {
				return err
			}
		} else if target.action != "" {
			result = "would remove" + strings.TrimPrefix(result, "removed")
			if target.result == "kept" {
				result = "kept (would remove generated header)"
			}
		}
		if _, err := fmt.Fprintf(out, "%s: %s\n", target.name, result); err != nil {
			return err
		}
	}
	if !yes {
		_, err = fmt.Fprintln(out, "NOTHING WAS REMOVED. Re-run with --yes to apply.")
	}
	return err
}
