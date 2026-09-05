// Package kittygfx implements the small subset of the Kitty terminal
// graphics protocol needed to show a background image behind the editor's
// text grid: transmitting a PNG, placing it behind the cell grid, and
// deleting it.
//
// Every emitted command carries q=2 (quiet): the protocol's response/error
// APC replies must never be requested here, because the host application's
// only read access to the tty is already owned by its terminal event loop.
package kittygfx

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"io"
	"math"
	"os"
)

// ImageID is the fixed Kitty image id used for the editor's background
// image. Only one background image placement exists at a time, so a single
// well-known id avoids any allocation/bookkeeping.
const ImageID = 1

const chunkSize = 4096

// DetectSupport reports whether the host terminal is likely to support the
// Kitty graphics protocol, based on environment variables set by known
// terminals. getenv is injected so this is testable without os.Setenv.
//
// This is deliberately best-effort: the protocol also defines an active
// capability query (a=q), but answering it requires reading a response off
// the tty, which would race the terminal library's own input reader.
func DetectSupport(getenv func(string) string) bool {
	if getenv("TERM") == "xterm-kitty" {
		return true
	}
	if getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	switch getenv("TERM_PROGRAM") {
	case "ghostty", "WezTerm":
		return true
	}
	return false
}

// EncodePNG loads the image at path, scales and crops it to cover exactly
// targetPxW x targetPxH pixels centered (like CSS "background-size: cover;
// background-position: center"), optionally dims it toward black by dim
// (0 = unchanged, 1 = fully black), and returns it re-encoded as PNG bytes.
//
// If targetPxW or targetPxH is <= 0 (the terminal did not report its pixel
// size), the image is left at its natural size instead of being cropped.
//
// The source is always re-decoded and re-encoded regardless of its original
// format, because Kitty's f=100 transmission format only decodes PNG.
func EncodePNG(path string, dim float64, targetPxW, targetPxH int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}

	if targetPxW > 0 && targetPxH > 0 {
		img = coverCrop(img, targetPxW, targetPxH)
	}
	if dim > 0 {
		img = dimImage(img, dim)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// coverCrop scales src so it fully covers a targetW x targetH box while
// preserving aspect ratio, then crops the overflow centered — equivalent to
// CSS "background-size: cover; background-position: center".
func coverCrop(src image.Image, targetW, targetH int) image.Image {
	bounds := src.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	if srcW == 0 || srcH == 0 {
		return src
	}

	scale := math.Max(float64(targetW)/float64(srcW), float64(targetH)/float64(srcH))
	scaledW := int(math.Round(float64(srcW) * scale))
	scaledH := int(math.Round(float64(srcH) * scale))
	if scaledW < targetW {
		scaledW = targetW
	}
	if scaledH < targetH {
		scaledH = targetH
	}

	offX := (scaledW - targetW) / 2
	offY := (scaledH - targetH) / 2

	dst := image.NewNRGBA(image.Rect(0, 0, targetW, targetH))
	for y := range targetH {
		sy := bounds.Min.Y + (y+offY)*srcH/scaledH
		for x := range targetW {
			sx := bounds.Min.X + (x+offX)*srcW/scaledW
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func dimImage(src image.Image, dim float64) image.Image {
	if dim > 1 {
		dim = 1
	}
	keep := 1 - dim
	bounds := src.Bounds()
	dst := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			dst.SetNRGBA(x, y, color.NRGBA{
				R: uint8(float64(c.R) * keep),
				G: uint8(float64(c.G) * keep),
				B: uint8(float64(c.B) * keep),
				A: c.A,
			})
		}
	}
	return dst
}

// Transmit sends PNG image data to the terminal under the given id, chunked
// per the Kitty graphics protocol's payload size limit. It does not place
// (display) the image; call Place afterwards.
func Transmit(w io.Writer, id uint32, pngData []byte) error {
	encoded := base64.StdEncoding.EncodeToString(pngData)

	for offset := 0; ; {
		end := offset + chunkSize
		last := end >= len(encoded)
		if last {
			end = len(encoded)
		}
		chunk := encoded[offset:end]

		more := 1
		if last {
			more = 0
		}

		var ctrl string
		if offset == 0 {
			ctrl = fmt.Sprintf("a=t,f=100,i=%d,q=2,m=%d", id, more)
		} else {
			ctrl = fmt.Sprintf("i=%d,q=2,m=%d", id, more)
		}
		if _, err := fmt.Fprintf(w, "\x1b_G%s;%s\x1b\\", ctrl, chunk); err != nil {
			return err
		}

		if last {
			return nil
		}
		offset = end
	}
}

// Place displays the previously transmitted image under id, scaled to fill
// cols x rows terminal cells anchored at the top-left corner, at a z-index
// behind normal text content.
//
// Without C=1, the protocol treats a placement like printed text: drawing it
// advances the cursor down by rows lines, which scrolls the screen (and
// shifts everything already rendered) once it runs past the bottom. C=1
// suppresses that. The cursor is also explicitly saved, homed to 1,1, and
// restored around the command so the placement always anchors at the
// top-left corner regardless of where the cursor happened to be, without
// leaving the terminal's cursor position out of sync with what the caller's
// screen library believes it left it at.
func Place(w io.Writer, id uint32, cols, rows int) error {
	_, err := fmt.Fprintf(w, "\x1b7\x1b[1;1H\x1b_Ga=p,i=%d,q=2,z=-1,C=1,c=%d,r=%d\x1b\\\x1b8", id, cols, rows)
	return err
}

// Delete removes the placement for id and frees its stored image data.
func Delete(w io.Writer, id uint32) error {
	_, err := fmt.Fprintf(w, "\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", id)
	return err
}
