package highlight

import (
	"path/filepath"
	"strings"
)

func NewForLines(filename string, lines []string) *Highlighter {
	c := currentCatalog()
	ref, ok := c.forLines(filename, lines)
	if !ok {
		return nil
	}
	return c.newHighlighter(ref)
}

func DetectLanguageID(filename string, lines []string) string {
	ref, ok := currentCatalog().forLines(filename, lines)
	if !ok {
		return ""
	}
	switch ref.languageID {
	case "tsx":
		return "typescriptreact"
	case "jsx":
		return "javascriptreact"
	default:
		return ref.languageID
	}
}

func (c *grammarCatalog) forLines(filename string, lines []string) (grammarRef, bool) {
	ref, ok := c.forFilename(filepath.Base(filename))
	if !ok && len(lines) > 0 {
		ref, ok = c.forLanguage(shebangLanguage(lines[0]))
	}
	return ref, ok
}

func shebangLanguage(line string) string {
	if !strings.HasPrefix(line, "#!") {
		return ""
	}
	fields := strings.Fields(line[2:])
	if len(fields) == 0 {
		return ""
	}
	interpreter := filepath.Base(fields[0])
	if interpreter == "env" {
		fields = fields[1:]
		for len(fields) > 0 {
			arg := fields[0]
			if arg != "-S" && arg != "--split-string" && arg != "-i" && arg != "--ignore-environment" && !strings.Contains(arg, "=") {
				break
			}
			fields = fields[1:]
		}
		if len(fields) == 0 || strings.HasPrefix(fields[0], "-") {
			return ""
		}
		interpreter = filepath.Base(fields[0])
	}
	switch interpreter {
	case "node", "nodejs":
		return "javascript"
	case "dash", "ksh":
		return "shellscript"
	}
	if strings.HasPrefix(interpreter, "python") {
		interpreter = strings.TrimRight(interpreter, "0123456789.")
	}
	return interpreter
}
