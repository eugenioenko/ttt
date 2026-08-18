package buffer

import (
	"bufio"
	"bytes"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// LoadFile loads a file into the buffer, replacing its contents.
func (b *Buffer) LoadFile(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	b.LineEnding = detectLineEnding(f)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), math.MaxInt)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	if b.ShowTrailingNewline && (len(lines) == 0 || lines[len(lines)-1] != "") {
		lines = append(lines, "")
	}
	b.Lines = lines
	b.Dirty = false
	if info, err := f.Stat(); err == nil {
		b.recordDiskInfo(info)
	}
	return nil
}

// detectLineEnding reads up to 64KB of a file to determine its line ending.
// Returns "\r\n" if CRLF is found, "\n" otherwise.
func detectLineEnding(r io.Reader) string {
	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	if n > 0 && bytes.Contains(buf[:n], []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

// recordDiskInfo stores the file's modification time and size so a later save
// can detect whether the file changed on disk in the meantime.
func (b *Buffer) recordDiskInfo(info os.FileInfo) {
	b.diskModTime = info.ModTime()
	b.diskSize = info.Size()
	b.diskInfoSet = true
}

// DiskChanged reports whether the file on disk has been modified since the
// buffer last loaded or saved it. It returns false when there is no recorded
// disk state (a new buffer never written to disk) or when the file no longer
// exists — a missing file is handled by the save itself, which recreates it.
func (b *Buffer) DiskChanged(filename string) bool {
	if !b.diskInfoSet {
		return false
	}
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.ModTime().Equal(b.diskModTime) || info.Size() != b.diskSize
}

// SaveFile writes the buffer contents to a file atomically: it writes to a
// temporary file in the same directory, fsyncs it, then renames it over the
// target. This guarantees the file on disk is always either the complete old
// or complete new content, never a truncated partial write. The target's
// permissions are preserved; symlinks are followed so the link is kept intact.
func (b *Buffer) SaveFile(filename string) error {
	lines := b.CleanedLines()

	// Resolve symlinks so we write through to the real file rather than
	// replacing the link itself with a regular file on rename.
	target := filename
	if resolved, err := filepath.EvalSymlinks(filename); err == nil {
		target = resolved
	}

	dir := filepath.Dir(target)
	mode := os.FileMode(0644)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(dir, ".ttt-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if we bail out before the rename succeeds.
	defer os.Remove(tmpName)

	w := bufio.NewWriter(tmp)
	if err := b.writeLines(w, lines); err != nil {
		tmp.Close()
		return err
	}
	if err := w.Flush(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpName, target); err != nil {
		return err
	}

	b.Dirty = false
	if info, err := os.Stat(target); err == nil {
		b.recordDiskInfo(info)
	}
	return nil
}

// CleanedLines returns Lines with the configured save-time cleanups applied.
// It never mutates the buffer: callers that want the editor to reflect the
// cleanup must apply it through an undo command, so a save stays undoable.
func (b *Buffer) CleanedLines() []string {
	lines := make([]string, len(b.Lines))
	copy(lines, b.Lines)

	if b.TrimTrailingWhitespace {
		for i, line := range lines {
			lines[i] = strings.TrimRight(line, " \t")
		}
	}
	if b.InsertFinalNewline && (len(lines) == 0 || lines[len(lines)-1] != "") {
		lines = append(lines, "")
	}
	if !b.InsertFinalNewline && b.ShowTrailingNewline && len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (b *Buffer) writeLines(w io.Writer, lines []string) error {
	eol := b.LineEnding
	if eol == "" {
		eol = "\n"
	}
	for i, line := range lines {
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
		if i < len(lines)-1 {
			if _, err := io.WriteString(w, eol); err != nil {
				return err
			}
		}
	}
	return nil
}
