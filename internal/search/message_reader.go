package search

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
)

const MaxRecordBytes = 16 * 1024 * 1024

var ErrRecordTooLarge = errors.New("search record exceeds 16 MiB limit")

type MessageReader struct {
	scanner *bufio.Scanner
}

func NewMessageReader(r io.Reader) *MessageReader {
	scanner := bufio.NewScanner(r)
	// rg ignores --max-columns in JSON mode. Allow one extra byte for the newline.
	scanner.Buffer(make([]byte, 256*1024), MaxRecordBytes+1)
	return &MessageReader{scanner: scanner}
}

func (r *MessageReader) Next() (RgMessage, error) {
	var msg RgMessage
	if !r.scanner.Scan() {
		err := r.scanner.Err()
		if errors.Is(err, bufio.ErrTooLong) {
			return msg, ErrRecordTooLarge
		}
		if err == nil {
			err = io.EOF
		}
		return msg, err
	}
	err := json.Unmarshal(r.scanner.Bytes(), &msg)
	return msg, err
}
