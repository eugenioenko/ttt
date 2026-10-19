package image

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"image"
	"image/color"
	"io"
	"math/rand/v2"
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

func inflatePayload(t *testing.T, payload string) []byte {
	t.Helper()
	deflated, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("payload not base64: %v", err)
	}
	zr, err := zlib.NewReader(bytes.NewReader(deflated))
	if err != nil {
		t.Fatalf("payload not zlib: %v", err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("inflate payload: %v", err)
	}
	return raw
}

func testSource(t *testing.T, w, h int) *Source {
	t.Helper()
	s, err := Decode(mustPNG(t, solidRGBA(w, h, color.RGBA{R: 10, G: 20, B: 30, A: 255})), "test.png")
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
	for k, want := range map[string]string{"a": "t", "f": "32", "o": "z", "t": "d", "s": "2", "v": "1", "m": "0"} {
		if p[k] != want {
			t.Errorf("param %s = %q, want %q (params %v)", k, p[k], want, p)
		}
	}
	if p["i"] == "" {
		t.Errorf("expected image id param, got %v", p)
	}
	if !bytes.Equal(inflatePayload(t, seqs[0].payload), s.Pix.Pix) {
		t.Error("payload does not round-trip to pixel bytes")
	}
}

// Noise barely compresses, so it still spans several chunks once deflated.
func noiseSource(t *testing.T, w, h int) *Source {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rng := rand.New(rand.NewPCG(1, 2))
	for i := range img.Pix {
		img.Pix[i] = byte(rng.UintN(256))
	}
	s, err := Decode(mustPNG(t, img), "noise.png")
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return s
}

func TestTransmitCompresses(t *testing.T) {
	s := testSource(t, 400, 300)
	var buf bytes.Buffer
	if err := Transmit(&buf, s); err != nil {
		t.Fatalf("Transmit: %v", err)
	}
	var payload strings.Builder
	for _, seq := range parseAPC(t, buf.Bytes()) {
		payload.WriteString(seq.payload)
	}
	if raw := base64.StdEncoding.EncodedLen(len(s.Pix.Pix)); payload.Len()*20 > raw {
		t.Errorf("flat image sent as %d bytes, want under 1/20 of the %d raw", payload.Len(), raw)
	}
	if !bytes.Equal(inflatePayload(t, payload.String()), s.Pix.Pix) {
		t.Error("payload does not round-trip to pixel bytes")
	}
}

func TestTransmitChunking(t *testing.T) {
	s := noiseSource(t, 64, 64)
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
	if !bytes.Equal(inflatePayload(t, combined.String()), s.Pix.Pix) {
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
