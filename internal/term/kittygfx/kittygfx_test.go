package kittygfx

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectSupport(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"kitty", map[string]string{"TERM": "xterm-kitty"}, true},
		{"kitty window id", map[string]string{"KITTY_WINDOW_ID": "1"}, true},
		{"ghostty", map[string]string{"TERM_PROGRAM": "ghostty"}, true},
		{"wezterm", map[string]string{"TERM_PROGRAM": "WezTerm"}, true},
		{"plain xterm", map[string]string{"TERM": "xterm-256color"}, false},
		{"unset", map[string]string{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(k string) string { return tc.env[k] }
			if got := DetectSupport(getenv); got != tc.want {
				t.Errorf("DetectSupport() = %v, want %v", got, tc.want)
			}
		})
	}
}

func writeTestJPEG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
}

func TestEncodePNG_JPEGInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bg.jpg")
	writeTestJPEG(t, path)

	out, err := EncodePNG(path, 0, 0, 0)
	if err != nil {
		t.Fatalf("EncodePNG: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("output is not a valid PNG: %v", err)
	}
}

func TestEncodePNG_Dim(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bg.jpg")
	writeTestJPEG(t, path)

	full, err := EncodePNG(path, 1.0, 0, 0)
	if err != nil {
		t.Fatalf("EncodePNG dim=1: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(full))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(0, 0).RGBA()
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("dim=1 should drive pixels to black, got r=%d g=%d b=%d", r, g, b)
	}

	none, err := EncodePNG(path, 0, 0, 0)
	if err != nil {
		t.Fatalf("EncodePNG dim=0: %v", err)
	}
	img2, err := png.Decode(bytes.NewReader(none))
	if err != nil {
		t.Fatal(err)
	}
	r2, g2, b2, _ := img2.At(0, 0).RGBA()
	if r2 == 0 && g2 == 0 && b2 == 0 {
		t.Errorf("dim=0 should leave pixels unchanged, got black")
	}
}

func TestEncodePNG_Cover(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bg.jpg")
	writeTestJPEG(t, path) // 4x4 square source

	// A wide target: covering it means the 4x4 source is scaled up so its
	// height fills 10px, then the width (also scaled to 20px) is cropped
	// down to the target's 16px, centered.
	out, err := EncodePNG(path, 0, 16, 10)
	if err != nil {
		t.Fatalf("EncodePNG: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() != 16 || b.Dy() != 10 {
		t.Errorf("cover-cropped size = %dx%d, want 16x10", b.Dx(), b.Dy())
	}
}

func TestTransmit_Chunking(t *testing.T) {
	var buf bytes.Buffer
	payload := bytes.Repeat([]byte{0xAB}, 10000)

	if err := Transmit(&buf, 7, payload); err != nil {
		t.Fatalf("Transmit: %v", err)
	}

	out := buf.String()
	parts := strings.Split(out, "\x1b\\")
	// Trailing empty string after the final terminator.
	var apcs []string
	for _, p := range parts {
		if p != "" {
			apcs = append(apcs, p+"\x1b\\")
		}
	}
	if len(apcs) < 2 {
		t.Fatalf("expected multiple chunks for a large payload, got %d", len(apcs))
	}
	if !strings.HasPrefix(apcs[0], "\x1b_Ga=t,f=100,i=7,q=2,m=1;") {
		t.Errorf("first chunk control data wrong: %q", apcs[0])
	}
	last := apcs[len(apcs)-1]
	if !strings.HasPrefix(last, "\x1b_Gi=7,q=2,m=0;") {
		t.Errorf("last chunk should have m=0, got %q", last)
	}
	for _, apc := range apcs[1 : len(apcs)-1] {
		if !strings.HasPrefix(apc, "\x1b_Gi=7,q=2,m=1;") {
			t.Errorf("continuation chunk should have m=1: %q", apc)
		}
	}
}

func TestPlace(t *testing.T) {
	var buf bytes.Buffer
	if err := Place(&buf, 7, 80, 24); err != nil {
		t.Fatalf("Place: %v", err)
	}
	want := "\x1b7\x1b[1;1H\x1b_Ga=p,i=7,q=2,z=-1,C=1,c=80,r=24\x1b\\\x1b8"
	if buf.String() != want {
		t.Errorf("Place() = %q, want %q", buf.String(), want)
	}
}

func TestDelete(t *testing.T) {
	var buf bytes.Buffer
	if err := Delete(&buf, 7); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	want := "\x1b_Ga=d,d=I,i=7,q=2\x1b\\"
	if buf.String() != want {
		t.Errorf("Delete() = %q, want %q", buf.String(), want)
	}
}
