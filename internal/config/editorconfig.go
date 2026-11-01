package config

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type EditorConfigProps struct {
	IndentStyle        string // "tab" or "space"
	IndentSize         int    // 0 means unset
	TrimTrailingWS     bool
	TrimTrailingWSSet  bool
	InsertFinalNewline bool
	InsertFinalNLSet   bool
}

type editorConfigSection struct {
	pattern string
	props   map[string]string
}

func LoadEditorConfig(filePath string) EditorConfigProps {
	var result EditorConfigProps
	dir := filepath.Dir(filePath)

	var files []string
	for {
		ec := filepath.Join(dir, ".editorconfig")
		if _, err := os.Stat(ec); err == nil {
			files = append(files, ec)
		}
		if isRoot(ec) {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Process from outermost to innermost (later overrides earlier)
	for i := len(files) - 1; i >= 0; i-- {
		sections := parseEditorConfigFile(files[i])
		rel, err := filepath.Rel(filepath.Dir(files[i]), filePath)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		for _, sec := range sections {
			if matchGlob(sec.pattern, rel) {
				applyProps(&result, sec.props)
			}
		}
	}

	return result
}

func isRoot(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		if line[0] == '[' {
			return false
		}
		k, v := parseKV(line)
		if strings.ToLower(k) == "root" && strings.ToLower(v) == "true" {
			return true
		}
	}
	return false
}

func parseEditorConfigFile(path string) []editorConfigSection {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var sections []editorConfigSection
	var current *editorConfigSection

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		if line[0] == '[' && line[len(line)-1] == ']' {
			pattern := line[1 : len(line)-1]
			sections = append(sections, editorConfigSection{
				pattern: pattern,
				props:   make(map[string]string),
			})
			current = &sections[len(sections)-1]
			continue
		}
		if current != nil {
			k, v := parseKV(line)
			if k != "" {
				current.props[strings.ToLower(k)] = v
			}
		}
	}
	return sections
}

func parseKV(line string) (string, string) {
	idx := strings.IndexAny(line, "=:")
	if idx < 0 {
		return "", ""
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:])
}

func applyProps(result *EditorConfigProps, props map[string]string) {
	if v, ok := props["indent_style"]; ok {
		result.IndentStyle = strings.ToLower(v)
	}
	if v, ok := props["indent_size"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			result.IndentSize = n
		}
	}
	if v, ok := props["trim_trailing_whitespace"]; ok {
		result.TrimTrailingWS = strings.ToLower(v) == "true"
		result.TrimTrailingWSSet = true
	}
	if v, ok := props["insert_final_newline"]; ok {
		result.InsertFinalNewline = strings.ToLower(v) == "true"
		result.InsertFinalNLSet = true
	}
}

// matchGlob matches a file's path, relative to the .editorconfig, against a
// section glob (https://spec.editorconfig.org/#glob-expressions).
// A glob without a slash matches the file name at any depth.
func matchGlob(pattern, path string) bool {
	if !strings.Contains(pattern, "/") {
		pattern = "**/" + pattern
	}
	var g globTranslator
	re, err := regexp.Compile("^" + g.translate(strings.TrimPrefix(pattern, "/")) + "$")
	if err != nil {
		return false
	}
	m := re.FindStringSubmatch(path)
	if m == nil {
		return false
	}
	for i, r := range g.ranges {
		if m[i+1] == "" {
			continue
		}
		n, err := strconv.Atoi(m[i+1])
		if err != nil || n < r[0] || n > r[1] {
			return false
		}
	}
	return true
}

var numRange = regexp.MustCompile(`^([+-]?\d+)\.\.([+-]?\d+)$`)

// globTranslator turns a glob into a regexp; each {num1..num2} becomes a
// capture group whose bounds are kept in ranges.
type globTranslator struct {
	ranges [][2]int
}

func (g *globTranslator) translate(p string) string {
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case '\\':
			if i+1 < len(p) {
				i++
			}
			b.WriteString(regexp.QuoteMeta(p[i : i+1]))
		case '*':
			switch {
			case strings.HasPrefix(p[i:], "**/"):
				b.WriteString("(?:.*/)?")
				i += 2
			case strings.HasPrefix(p[i:], "**"):
				b.WriteString(".*")
				i++
			default:
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '[':
			end := strings.IndexByte(p[i+1:], ']')
			if end <= 0 || strings.Contains(p[i+1:i+1+end], "/") {
				b.WriteString(`\[`)
				continue
			}
			class := p[i+1 : i+1+end]
			i += end + 1
			b.WriteByte('[')
			if strings.HasPrefix(class, "!") {
				b.WriteByte('^')
				class = class[1:]
			}
			b.WriteString(strings.NewReplacer(`\`, `\\`, "[", `\[`).Replace(class))
			b.WriteByte(']')
		case '{':
			end := closingBrace(p, i)
			if end < 0 {
				b.WriteString(`\{`)
				continue
			}
			inner := p[i+1 : end]
			i = end
			if m := numRange.FindStringSubmatch(inner); m != nil {
				lo, _ := strconv.Atoi(m[1])
				hi, _ := strconv.Atoi(m[2])
				g.ranges = append(g.ranges, [2]int{lo, hi})
				b.WriteString(`([+-]?(?:0|[1-9]\d*))`)
			} else if alts := splitAlternatives(inner); len(alts) > 1 {
				for k, alt := range alts {
					alts[k] = g.translate(alt)
				}
				b.WriteString("(?:" + strings.Join(alts, "|") + ")")
			} else {
				b.WriteString(`\{` + g.translate(inner) + `\}`)
			}
		default:
			b.WriteString(regexp.QuoteMeta(p[i : i+1]))
		}
	}
	return b.String()
}

// closingBrace returns the index of the '}' that closes the '{' at open, or -1.
func closingBrace(p string, open int) int {
	depth := 0
	for i := open; i < len(p); i++ {
		switch p[i] {
		case '\\':
			i++
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// splitAlternatives splits a brace body at its top-level commas.
func splitAlternatives(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '{':
			depth++
		case '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}
