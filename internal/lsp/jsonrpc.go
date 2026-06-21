package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int   `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (r *Response) IsNotification() bool {
	return r.ID == nil && r.Method != ""
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}

type Codec struct {
	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex
}

func NewCodec(r io.Reader, w io.Writer) *Codec {
	return &Codec{
		reader: bufio.NewReader(r),
		writer: w,
	}
}

func (c *Codec) Send(req Request) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := io.WriteString(c.writer, header); err != nil {
		return err
	}
	_, err = c.writer.Write(data)
	return err
}

func (c *Codec) Receive() (Response, error) {
	contentLen := -1
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return Response{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if val, ok := strings.CutPrefix(line, "Content-Length: "); ok {
			n, err := strconv.Atoi(val)
			if err != nil {
				return Response{}, fmt.Errorf("bad Content-Length: %w", err)
			}
			contentLen = n
		}
	}
	if contentLen < 0 {
		return Response{}, fmt.Errorf("missing Content-Length header")
	}
	body := make([]byte, contentLen)
	if _, err := io.ReadFull(c.reader, body); err != nil {
		return Response{}, err
	}
	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		return Response{}, err
	}
	return resp, nil
}
