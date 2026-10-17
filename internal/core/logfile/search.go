package logfile

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxMatches = 10000
const MaxCopyBytes = 4 << 20

type Match struct {
	Offset   int64
	Col, Len int
}
type SearchResult struct {
	Start         int64
	Matches       []Match
	Scanned, Size int64
	Limited       bool
}

func (lf *File) Search(ctx context.Context, re *regexp.Regexp, progress func(SearchResult)) (SearchResult, error) {
	return lf.SearchFrom(ctx, re, 0, progress)
}

func (lf *File) SearchFrom(ctx context.Context, re *regexp.Regexp, start int64, progress func(SearchResult)) (SearchResult, error) {
	lf.mu.Lock()
	if lf.closed {
		lf.mu.Unlock()
		return SearchResult{}, io.ErrClosedPipe
	}
	f, gen, size := lf.f, lf.gen, lf.size
	lf.mu.Unlock()
	start, err := searchLineStart(ctx, f, min(max(start, 0), size))
	if err != nil {
		return SearchResult{}, err
	}
	result := SearchResult{Start: start, Scanned: start, Size: size}
	// The consumed context rune preserves ^ and word-boundary semantics when
	// resuming after a match, without retaining arbitrarily long log entries.
	resumed, err := regexp.Compile(`\A(?s:..*?)(` + re.String() + `)`)
	if err != nil {
		return result, err
	}
	buffer := bufio.NewReaderSize(io.NewSectionReader(f, start, size-start), chunkSize)
	lastNotice := time.Time{}
	bufferOffset := start
	check := func(off int64) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		lf.mu.Lock()
		valid := lf.gen == gen && !lf.closed
		lf.mu.Unlock()
		if !valid {
			return errors.New("file changed during search")
		}
		result.Scanned = max(result.Scanned, off)
		if progress != nil && time.Since(lastNotice) >= 50*time.Millisecond {
			update := result
			update.Matches = append([]Match(nil), result.Matches...)
			progress(update)
			lastNotice = time.Now()
		}
		return nil
	}
	lineStart := start
	for lineStart < size || size == 0 && lineStart == 0 {
		searchStart := lineStart
		previousEnd := int64(-1)
		colBase := 0
		colOffset := lineStart
		for {
			if err := check(searchStart); err != nil {
				return result, err
			}
			inputStart := searchStart
			pattern := re
			group := 0
			if searchStart > lineStart {
				var previous [utf8.UTFMax]byte
				from := max(searchStart-utf8.UTFMax, lineStart)
				n, _ := f.ReadAt(previous[:searchStart-from], from)
				_, width := utf8.DecodeLastRune(previous[:n])
				inputStart -= int64(width)
				pattern = resumed
				group = 2
			}
			if inputStart != bufferOffset {
				buffer.Reset(io.NewSectionReader(f, inputStart, size-inputStart))
			}
			reader := &searchLineReader{reader: buffer, offset: inputStart, check: check}
			loc := pattern.FindReaderSubmatchIndex(reader)
			if len(loc) == 0 {
				// Anchored expressions can reject a line before consuming its newline.
				for !reader.ended && reader.err == nil {
					reader.ReadRune()
				}
			}
			bufferOffset = reader.offset
			if reader.err != nil {
				return result, reader.err
			}
			if len(loc) == 0 {
				lineStart = reader.next
				if lineStart >= size {
					result.Scanned = size
					return result, nil
				}
				break
			}
			start, end := inputStart+int64(loc[group]), inputStart+int64(loc[group+1])
			if start != end || end != previousEnd {
				if len(result.Matches) == MaxMatches {
					result.Limited = true
					return result, nil
				}
				colBase += countRunes(f, colOffset, start)
				result.Matches = append(result.Matches, Match{Offset: lineStart, Col: colBase, Len: countRunes(f, start, end)})
				colOffset = start
			}
			previousEnd = end
			searchStart = end
			if start == end {
				buffer.Reset(io.NewSectionReader(f, end, size-end))
				ch, width, err := buffer.ReadRune()
				if err != nil {
					result.Scanned = size
					return result, nil
				}
				if ch == '\r' {
					if next, _ := buffer.Peek(1); len(next) > 0 && next[0] == '\n' {
						width++
						buffer.Discard(1)
						ch = '\n'
					}
				}
				if ch == '\n' {
					lineStart = end + int64(width)
					if lineStart >= size {
						result.Scanned = size
						return result, nil
					}
					break
				}
				searchStart += int64(width)
				bufferOffset = searchStart
			}
		}
	}
	return result, nil
}

func searchLineStart(ctx context.Context, f *os.File, offset int64) (int64, error) {
	buf := make([]byte, chunkSize)
	for offset > 0 {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		from := max(offset-chunkSize, 0)
		n, err := f.ReadAt(buf[:offset-from], from)
		if err != nil {
			return 0, err
		}
		if i := bytes.LastIndexByte(buf[:n], '\n'); i >= 0 {
			return from + int64(i) + 1, nil
		}
		offset = from
	}
	return 0, nil
}

type searchLineReader struct {
	reader       *bufio.Reader
	offset, next int64
	check        func(int64) error
	err          error
	count        int
	ended        bool
}

func (r *searchLineReader) ReadRune() (rune, int, error) {
	if r.err != nil {
		return 0, 0, r.err
	}
	if r.ended {
		return 0, 0, io.EOF
	}
	if r.count%4096 == 0 {
		if r.err = r.check(r.offset); r.err != nil {
			return 0, 0, r.err
		}
	}
	ch, width, err := r.reader.ReadRune()
	r.offset += int64(width)
	r.count++
	if ch == '\r' {
		if next, _ := r.reader.Peek(1); len(next) > 0 && next[0] == '\n' {
			r.reader.Discard(1)
			r.offset++
			ch = '\n'
		}
	}
	if ch == '\n' || err != nil {
		r.next = r.offset
		r.ended = true
		if err != nil && err != io.EOF {
			r.err = err
		}
		return 0, 0, io.EOF
	}
	return ch, width, nil
}
func countRunes(f *os.File, start, end int64) int {
	reader := bufio.NewReaderSize(io.NewSectionReader(f, start, max(end-start, 0)), 4096)
	n := 0
	for {
		if _, _, err := reader.ReadRune(); err != nil {
			return n
		}
		n++
	}
}

type Position struct {
	Offset int64
	Col    int
}

func Before(a, b Position) bool { return a.Offset < b.Offset || a.Offset == b.Offset && a.Col < b.Col }

func (lf *File) Copy(start, end Position) (string, bool, error) {
	if Before(end, start) {
		start, end = end, start
	}
	var out strings.Builder
	off := start.Offset
	for off <= end.Offset && off < lf.Progress().Size {
		row, err := lf.line(off)
		if err != nil {
			return out.String(), false, err
		}
		text := []rune(row.Text)
		lo, hi := 0, len(text)
		if off == start.Offset {
			lo = min(start.Col, len(text))
		}
		if off == end.Offset {
			hi = min(end.Col, len(text))
		}
		part := string(text[lo:max(lo, hi)])
		if off < end.Offset && (row.Next < lf.Progress().Size || lf.endsWithNewline()) {
			part += "\n"
		}
		remaining := MaxCopyBytes - out.Len()
		if len(part) > remaining {
			part = part[:remaining]
			for !utf8.ValidString(part) {
				part = part[:len(part)-1]
			}
			out.WriteString(part)
			return out.String(), true, nil
		}
		out.WriteString(part)
		if row.Truncated {
			return out.String(), true, nil
		}
		if off == end.Offset || row.Next <= off {
			break
		}
		off = row.Next
	}
	return out.String(), false, nil
}

func (lf *File) endsWithNewline() bool {
	size := lf.Progress().Size
	if size == 0 {
		return false
	}
	data, err := lf.readChunk(size - 1)
	return err == nil && len(data) > int((size-1)%chunkSize) && data[(size-1)%chunkSize] == '\n'
}
