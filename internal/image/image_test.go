package image

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func solidRGBA(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func mustPNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func pngWithIHDR(width, height uint32) []byte {
	var buf bytes.Buffer
	buf.WriteString("\x89PNG\r\n\x1a\n")
	var ihdr bytes.Buffer
	binary.Write(&ihdr, binary.BigEndian, width)
	binary.Write(&ihdr, binary.BigEndian, height)
	ihdr.Write([]byte{8, 2, 0, 0, 0})
	chunk := func(typ string, data []byte) {
		binary.Write(&buf, binary.BigEndian, uint32(len(data)))
		buf.WriteString(typ)
		buf.Write(data)
		crc := crc32.NewIEEE()
		crc.Write([]byte(typ))
		crc.Write(data)
		binary.Write(&buf, binary.BigEndian, crc.Sum32())
	}
	chunk("IHDR", ihdr.Bytes())
	return buf.Bytes()
}

func TestDecodeFormats(t *testing.T) {
	src := solidRGBA(4, 3, color.RGBA{R: 255, A: 255})

	var jpgBuf bytes.Buffer
	if err := jpeg.Encode(&jpgBuf, src, nil); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	var gifBuf bytes.Buffer
	if err := gif.Encode(&gifBuf, src, nil); err != nil {
		t.Fatalf("gif.Encode: %v", err)
	}

	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"png", mustPNG(t, src)},
		{"jpeg", jpgBuf.Bytes()},
		{"gif", gifBuf.Bytes()},
	} {
		s, err := decode(tc.data, "test."+tc.name)
		if err != nil {
			t.Errorf("%s: Decode: %v", tc.name, err)
			continue
		}
		if s.W != 4 || s.H != 3 {
			t.Errorf("%s: expected 4x3, got %dx%d", tc.name, s.W, s.H)
		}
		if s.Pix == nil {
			t.Errorf("%s: expected non-nil Pix", tc.name)
		}
		if s.Path != "test."+tc.name {
			t.Errorf("%s: expected Path preserved, got %q", tc.name, s.Path)
		}
		if s.ID == 0 {
			t.Errorf("%s: expected nonzero ID", tc.name)
		}
	}
}

func TestDecodeIDsUnique(t *testing.T) {
	data := mustPNG(t, solidRGBA(2, 2, color.RGBA{G: 255, A: 255}))
	a, err := decode(data, "a.png")
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	b, err := decode(data, "b.png")
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if a.ID == b.ID {
		t.Errorf("expected unique IDs, both %d", a.ID)
	}
}

func TestLoadRoundTrip(t *testing.T) {
	data := mustPNG(t, solidRGBA(5, 6, color.RGBA{B: 255, A: 255}))
	path := filepath.Join(t.TempDir(), "img.png")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.W != 5 || s.H != 6 {
		t.Errorf("expected 5x6, got %dx%d", s.W, s.H)
	}
	if s.Path != path {
		t.Errorf("expected Path %q, got %q", path, s.Path)
	}
}

func TestLoadMissing(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.png")); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadRejectsHugeFileBeforeReading(t *testing.T) {
	path := filepath.Join(t.TempDir(), "huge.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// Sparse: no bytes are written, so a read-first loader would allocate 64 MiB+.
	if err := f.Truncate(maxImageFileBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = Load(path)
	if err == nil || !strings.Contains(err.Error(), "exceeds maximum") {
		t.Fatalf("expected size rejection, got %v", err)
	}
}

func TestLoadRejectsOversized(t *testing.T) {
	for _, tc := range []struct {
		name       string
		w, h       uint32
		wantSubstr string
	}{
		{"dimension", 20000, 10, "dimensions"},
		{"pixels", 8000, 8000, "pixel count"},
	} {
		path := filepath.Join(t.TempDir(), tc.name+".png")
		if err := os.WriteFile(path, pngWithIHDR(tc.w, tc.h), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		_, err := Load(path)
		if err == nil {
			t.Errorf("%s: expected rejection", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantSubstr) {
			t.Errorf("%s: error %q should mention %q", tc.name, err, tc.wantSubstr)
		}
	}
}

func TestDecodeCorrupt(t *testing.T) {
	valid := mustPNG(t, solidRGBA(4, 4, color.RGBA{A: 255}))
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"garbage", []byte("this is not an image")},
		{"empty", nil},
		{"truncated", valid[:len(valid)/2]},
		{"one-byte", []byte{0x89}},
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: panicked: %v", tc.name, r)
				}
			}()
			if _, err := decode(tc.data, tc.name); err == nil {
				t.Errorf("%s: expected error", tc.name)
			}
		}()
	}
}
