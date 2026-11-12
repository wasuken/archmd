package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func promptProject(dir string, maxLines int, out io.Writer) error {
	rules, err := agentsRules(maxLines)
	if err != nil {
		return err
	}
	_, err = os.Stat(filepath.Join(dir, ".architecture"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var prompt strings.Builder
	prompt.WriteString("Update ARCHITECTURE.md so it reflects the current state of this codebase.\n\n")
	if err == nil {
		prompt.WriteString("Edit the files in `.architecture/`, then run `archmd build`. Do not edit ARCHITECTURE.md directly.\n\n")
	} else {
		prompt.WriteString("Edit only the affected sections of ARCHITECTURE.md directly.\n\n")
	}
	prompt.WriteString(rules)
	prompt.WriteByte('\n')
	var diagnostics bytes.Buffer
	checkProject(dir, maxLines, 80, &diagnostics)
	missing := false
	for _, line := range strings.Split(diagnostics.String(), "\n") {
		if strings.HasPrefix(line, "ERROR [") || strings.HasPrefix(line, "WARN [") || strings.HasPrefix(line, "INFO [") || strings.HasPrefix(line, "lines: ") {
			prompt.WriteString(line + "\n")
		}
		if strings.HasPrefix(line, "ERROR [no-architecture-file]") {
			missing = true
		}
	}
	if missing {
		prompt.WriteString("Run `archmd init` first to create ARCHITECTURE.md.\n")
	}
	prompt.WriteString("\nYou are done when `archmd check` exits with code 0.\n")
	_, err = io.WriteString(out, prompt.String())
	return err
}
