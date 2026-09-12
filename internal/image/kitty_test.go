package image

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"strings"
	"testing"
)

type apcSeq struct {
	params  map[string]string
	payload string
}

func parseAPC(t *testing.T, out []byte) []apcSeq {
	t.Helper()
	var seqs []apcSeq
	rest := out
	for len(rest) > 0 {
		start := bytes.Index(rest, []byte("\x1b_G"))
		if start < 0 {
			t.Fatalf("trailing non-APC bytes: %q", rest)
		}
		if start > 0 {
			t.Fatalf("leading non-APC bytes: %q", rest[:start])
		}
		rest = rest[len("\x1b_G"):]
		end := bytes.Index(rest, []byte("\x1b\\"))
		if end < 0 {
			t.Fatalf("unterminated APC sequence")
		}
		body := string(rest[:end])
		rest = rest[end+len("\x1b\\"):]
		semi := strings.Index(body, ";")
		if semi < 0 {
			t.Fatalf("APC body without ; separator: %q", body)
		}
		seq := apcSeq{params: map[string]string{}, payload: body[semi+1:]}
		for _, kv := range strings.Split(body[:semi], ",") {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				t.Fatalf("param without =: %q", kv)
			}
			seq.params[k] = v
		}
		seqs = append(seqs, seq)
	}
	return seqs
}

func testSource(t *testing.T, w, h int) *Source {
	t.Helper()
	s, err := decode(mustPNG(t, solidRGBA(w, h, color.RGBA{R: 10, G: 20, B: 30, A: 255})), "test.png")
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return s
}

func TestTransmitSmall(t *testing.T) {
	s := testSource(t, 2, 1)
	var buf bytes.Buffer
	if err := Transmit(&buf, s); err != nil {
		t.Fatalf("Transmit: %v", err)
	}
	seqs := parseAPC(t, buf.Bytes())
	if len(seqs) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(seqs))
	}
	p := seqs[0].params
	for k, want := range map[string]string{"a": "t", "f": "32", "t": "d", "s": "2", "v": "1", "m": "0"} {
		if p[k] != want {
			t.Errorf("param %s = %q, want %q (params %v)", k, p[k], want, p)
		}
	}
	if p["i"] == "" {
		t.Errorf("expected image id param, got %v", p)
	}
	raw, err := base64.StdEncoding.DecodeString(seqs[0].payload)
	if err != nil {
		t.Fatalf("payload not base64: %v", err)
	}
	if !bytes.Equal(raw, s.Pix.Pix) {
		t.Error("payload does not round-trip to pixel bytes")
	}
}

func TestTransmitChunking(t *testing.T) {
	s := testSource(t, 64, 64)
	var buf bytes.Buffer
	if err := Transmit(&buf, s); err != nil {
		t.Fatalf("Transmit: %v", err)
	}
	seqs := parseAPC(t, buf.Bytes())
	if len(seqs) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(seqs))
	}
	var combined strings.Builder
	for i, seq := range seqs {
		if len(seq.payload) > kittyChunkSize {
			t.Errorf("chunk %d payload %d bytes exceeds %d", i, len(seq.payload), kittyChunkSize)
		}
		wantM := "1"
		if i == len(seqs)-1 {
			wantM = "0"
		}
		if seq.params["m"] != wantM {
			t.Errorf("chunk %d m = %q, want %q", i, seq.params["m"], wantM)
		}
		combined.WriteString(seq.payload)
	}
	raw, err := base64.StdEncoding.DecodeString(combined.String())
	if err != nil {
		t.Fatalf("combined payload not base64: %v", err)
	}
	if !bytes.Equal(raw, s.Pix.Pix) {
		t.Error("combined chunks do not round-trip to pixel bytes")
	}
}

func TestTransmitNil(t *testing.T) {
	var buf bytes.Buffer
	if err := Transmit(&buf, nil); err == nil {
		t.Error("expected error for nil source")
	}
	if buf.Len() != 0 {
		t.Error("expected zero bytes on error")
	}
}

func TestPlace(t *testing.T) {
	p := Placement{
		SourceID:    42,
		PlacementID: 7,
		Cell:        Rect{X: 2, Y: 3, W: 4, H: 5},
		Src:         image.Rect(1, 2, 10, 12),
		Z:           -1,
	}
	var buf bytes.Buffer
	if err := Place(&buf, p); err != nil {
		t.Fatalf("Place: %v", err)
	}
	out := buf.Bytes()
	if !bytes.HasPrefix(out, []byte("\x1b[4;3H")) {
		t.Errorf("expected CUP to row 4 col 3, got %q", out)
	}
	seqs := parseAPC(t, out[bytes.Index(out, []byte("\x1b_G")):])
	if len(seqs) != 1 {
		t.Fatalf("expected 1 sequence, got %d", len(seqs))
	}
	params := seqs[0].params
	for k, want := range map[string]string{
		"a": "p", "i": "42", "p": "7", "C": "1",
		"x": "1", "y": "2", "w": "9", "h": "10",
		"c": "4", "r": "5", "z": "-1",
	} {
		if params[k] != want {
			t.Errorf("param %s = %q, want %q (params %v)", k, params[k], want, params)
		}
	}
}

func TestDelete(t *testing.T) {
	var buf bytes.Buffer
	if err := DeletePlacement(&buf, 42, 7); err != nil {
		t.Fatalf("DeletePlacement: %v", err)
	}
	seqs := parseAPC(t, buf.Bytes())
	if len(seqs) != 1 {
		t.Fatalf("expected 1 sequence, got %d", len(seqs))
	}
	p := seqs[0].params
	for k, want := range map[string]string{"a": "d", "d": "i", "i": "42", "p": "7"} {
		if p[k] != want {
			t.Errorf("param %s = %q, want %q", k, p[k], want)
		}
	}

	buf.Reset()
	if err := DeleteImage(&buf, 42); err != nil {
		t.Fatalf("DeleteImage: %v", err)
	}
	seqs = parseAPC(t, buf.Bytes())
	if len(seqs) != 1 {
		t.Fatalf("expected 1 sequence, got %d", len(seqs))
	}
	p = seqs[0].params
	for k, want := range map[string]string{"a": "d", "d": "I", "i": "42"} {
		if p[k] != want {
			t.Errorf("param %s = %q, want %q", k, p[k], want)
		}
	}
}
