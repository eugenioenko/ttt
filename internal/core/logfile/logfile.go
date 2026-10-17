package logfile

import (
	"bytes"
	"errors"
	"io"
	"os"
	"sort"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	Stride       = 256
	chunkSize    = 64 << 10
	MaxLineBytes = 1 << 20
	cacheChunks  = 8
)

type Progress struct {
	Scanned, Size int64
	Lines         int
	Indexing      bool
	Generation    uint64
	Err           error
}

type Row struct {
	Offset, Next int64
	Text         string
	Truncated    bool
}

type cachedChunk struct {
	offset int64
	data   []byte
}
type longLine struct{ start, end int64 }

// ReadAt avoids SIGBUS when a writer truncates a file while it is open.
type File struct {
	path                      string
	onProgress                func()
	mu                        sync.Mutex
	f                         *os.File
	gen                       uint64
	marks                     []int64
	longLines                 []longLine
	lines                     int
	indexed, scanned, size    int64
	running, replaced, closed bool
	err                       error
	cache                     []cachedChunk
	workers                   sync.WaitGroup
}

func Open(path string, onProgress func()) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		if err == nil {
			err = errors.New("not a regular file")
		}
		return nil, err
	}
	lf := &File{path: path, f: f, onProgress: onProgress, marks: []int64{0}, size: info.Size()}
	lf.startLocked()
	return lf, nil
}

func (lf *File) Path() string { return lf.path }
func (lf *File) Progress() Progress {
	lf.mu.Lock()
	defer lf.mu.Unlock()
	n := lf.lines
	if lf.scanned > lf.indexed {
		n++
	}
	return Progress{Scanned: lf.scanned, Size: lf.size, Lines: max(n, 1), Indexing: lf.running, Generation: lf.gen, Err: lf.err}
}
func (lf *File) Lines() int     { return lf.Progress().Lines }
func (lf *File) Indexing() bool { return lf.Progress().Indexing }
func (lf *File) TakeReplaced() bool {
	lf.mu.Lock()
	defer lf.mu.Unlock()
	r := lf.replaced
	lf.replaced = false
	return r
}

func (lf *File) Refresh() error {
	info, err := os.Stat(lf.path)
	if err != nil {
		return err
	}
	lf.mu.Lock()
	if lf.closed {
		lf.mu.Unlock()
		return os.ErrClosed
	}
	opened, err := lf.f.Stat()
	if err != nil {
		lf.mu.Unlock()
		return err
	}
	if !os.SameFile(opened, info) || info.Size() < max(lf.size, lf.scanned) {
		f, err := os.Open(lf.path)
		if err != nil {
			lf.mu.Unlock()
			return err
		}
		lf.f.Close()
		lf.f = f
		lf.gen++
		lf.marks, lf.longLines, lf.lines, lf.indexed, lf.scanned = []int64{0}, nil, 0, 0, 0
		lf.running = false
		lf.replaced = true
	}
	lf.size = info.Size()
	lf.cache = nil
	if !lf.running && (lf.scanned < lf.size || lf.replaced) {
		lf.startLocked()
	}
	lf.mu.Unlock()
	lf.notify()
	return nil
}

func (lf *File) Close() {
	lf.mu.Lock()
	if !lf.closed {
		lf.closed = true
		lf.gen++
		lf.f.Close()
		lf.cache = nil
	}
	lf.mu.Unlock()
	lf.workers.Wait()
}
func (lf *File) notify() {
	if lf.onProgress != nil {
		lf.onProgress()
	}
}
func (lf *File) startLocked() {
	lf.running = true
	lf.err = nil
	lf.workers.Add(1)
	go lf.index(lf.gen, lf.f, lf.scanned, lf.indexed, lf.lines)
}
func (lf *File) index(gen uint64, f *os.File, off, lastNL int64, lines int) {
	defer lf.workers.Done()
	buf := make([]byte, 1<<20)
	lastNotice := time.Time{}
	for {
		lf.mu.Lock()
		if lf.closed || lf.gen != gen {
			lf.mu.Unlock()
			return
		}
		remaining := max(lf.size-off, 0)
		lf.mu.Unlock()
		n, err := f.ReadAt(buf[:min(int64(len(buf)), remaining)], off)
		data := buf[:n]
		pos := off
		var marks []int64
		var longs []longLine
		for {
			i := bytes.IndexByte(data, '\n')
			if i < 0 {
				break
			}
			pos += int64(i) + 1
			if pos-lastNL > MaxLineBytes {
				longs = append(longs, longLine{lastNL, pos})
			}
			lastNL = pos
			data = data[i+1:]
			lines++
			if lines%Stride == 0 {
				marks = append(marks, pos)
			}
		}
		off += int64(n)
		lf.mu.Lock()
		if lf.closed || lf.gen != gen {
			lf.mu.Unlock()
			return
		}
		lf.marks = append(lf.marks, marks...)
		lf.longLines = append(lf.longLines, longs...)
		lf.lines, lf.indexed, lf.scanned = lines, lastNL, off
		done := off >= lf.size || err != nil
		if done && lf.size > off && err == nil && n > 0 {
			done = false
		}
		if done {
			lf.running = false
			if err != nil && !errors.Is(err, io.EOF) {
				lf.err = err
			}
		}
		lf.mu.Unlock()
		if done || time.Since(lastNotice) >= 50*time.Millisecond {
			lf.notify()
			lastNotice = time.Now()
		}
		if done {
			return
		}
	}
}

func (lf *File) readChunk(off int64) ([]byte, error) {
	base := off / int64(chunkSize) * int64(chunkSize)
	lf.mu.Lock()
	defer lf.mu.Unlock()
	if lf.closed {
		return nil, os.ErrClosed
	}
	for i, c := range lf.cache {
		if c.offset == base {
			copy(lf.cache[1:i+1], lf.cache[:i])
			lf.cache[0] = c
			return c.data, nil
		}
	}
	data := make([]byte, chunkSize)
	n, err := lf.f.ReadAt(data, base)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	data = data[:n]
	lf.cache = append(lf.cache, cachedChunk{})
	copy(lf.cache[1:], lf.cache[:len(lf.cache)-1])
	lf.cache[0] = cachedChunk{base, data}
	if len(lf.cache) > cacheChunks {
		lf.cache = lf.cache[:cacheChunks]
	}
	return data, nil
}

func (lf *File) line(off int64) (Row, error) {
	row := Row{Offset: off, Next: off}
	size := lf.Progress().Size
	var text []byte
	for off < size {
		data, err := lf.readChunk(off)
		if err != nil {
			return row, err
		}
		pos := int(off % chunkSize)
		if pos >= len(data) {
			break
		}
		data = data[pos:]
		i := bytes.IndexByte(data, '\n')
		part := data
		if i >= 0 {
			part = data[:i]
		}
		take := min(len(part), MaxLineBytes-len(text))
		text = append(text, part[:take]...)
		row.Truncated = row.Truncated || take < len(part)
		off += int64(len(part))
		if i >= 0 {
			off++
			break
		}
		if len(text) == MaxLineBytes && off >= size {
			break
		}
		if len(text) == MaxLineBytes {
			next, err := lf.readChunk(off)
			pos := int(off % chunkSize)
			if err == nil && pos < len(next) && next[pos] == '\n' {
				off++
				break
			}
			lf.mu.Lock()
			k := sort.Search(len(lf.longLines), func(i int) bool { return lf.longLines[i].start >= row.Offset })
			if k < len(lf.longLines) && lf.longLines[k].start == row.Offset {
				off = lf.longLines[k].end
			} else if !lf.running {
				off = size
			} else {
				// The index will supply the next line's offset without scanning a
				// multi-gigabyte line on the UI goroutine.
				off = -1
			}
			lf.mu.Unlock()
			row.Truncated = true
			break
		}
	}
	row.Text = displayLine(text)
	row.Next = off
	return row, nil
}

func (lf *File) ReadRows(offset int64, n int) ([]Row, error) {
	if n <= 0 || offset < 0 {
		return nil, nil
	}
	p := lf.Progress()
	rows := make([]Row, 0, n)
	bytesRead := 0
	for len(rows) < n && (offset < p.Size || offset == 0 && len(rows) == 0) {
		row, err := lf.line(offset)
		if err != nil {
			return rows, err
		}
		rows = append(rows, row)
		bytesRead += len(row.Text)
		if row.Next <= offset || bytesRead >= 2*MaxLineBytes {
			break
		}
		offset = row.Next
	}
	return rows, nil
}

func (lf *File) OffsetForLine(line int) (int64, bool) {
	lf.mu.Lock()
	if line < 0 || line >= lf.lines && !(line == lf.lines && lf.scanned > lf.indexed) && line != 0 {
		lf.mu.Unlock()
		return 0, false
	}
	k := min(line/Stride, len(lf.marks)-1)
	off := lf.marks[k]
	lf.mu.Unlock()
	for n := k * Stride; n < line; n++ {
		row, err := lf.line(off)
		if err != nil || row.Next <= off {
			return 0, false
		}
		off = row.Next
	}
	return off, true
}

func (lf *File) LineNumber(offset int64) (int, bool) {
	lf.mu.Lock()
	if offset > lf.scanned || offset == lf.scanned && lf.running && offset > 0 {
		lf.mu.Unlock()
		return 0, false
	}
	k := sort.Search(len(lf.marks), func(i int) bool { return lf.marks[i] > offset }) - 1
	k = max(k, 0)
	off := lf.marks[k]
	lf.mu.Unlock()
	line := k * Stride
	for off < offset {
		row, err := lf.line(off)
		if err != nil || row.Next <= off || row.Next > offset {
			return line, err == nil
		}
		off = row.Next
		line++
	}
	return line, true
}

// Previous returns an offset even while the forward index is still incomplete.
func (lf *File) Previous(offset int64, n int) int64 {
	if n <= 0 {
		return offset
	}
	off := max(offset-1, 0)
	remaining := n
	for off > 0 && remaining > 0 {
		base := (off - 1) / chunkSize * chunkSize
		data, err := lf.readChunk(base)
		if err != nil {
			return 0
		}
		data = data[:min(int(off-base), len(data))]
		for len(data) > 0 {
			i := bytes.LastIndexByte(data, '\n')
			if i < 0 {
				break
			}
			remaining--
			if remaining == 0 {
				return base + int64(i) + 1
			}
			data = data[:i]
		}
		off = base
		if offset-off > 2*MaxLineBytes {
			return off
		}
	}
	return 0
}

func (lf *File) Tail(n int) int64 {
	p := lf.Progress()
	return lf.Previous(p.Size, max(n, 1))
}

func (lf *File) ReadLines(start, n int) []string {
	off, ok := lf.OffsetForLine(start)
	if !ok {
		return nil
	}
	rows, _ := lf.ReadRows(off, n)
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.Text
	}
	return out
}
func displayLine(b []byte) string {
	b = bytes.TrimSuffix(b, []byte{'\r'})
	if utf8.Valid(b) {
		return string(b)
	}
	return string(bytes.Runes(b))
}
