package image

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"sync/atomic"
	"time"

	randv2 "math/rand/v2"
)

const (
	maxImageDimension = 16384
	maxImagePixels    = 40_000_000
	maxImageFileBytes = 64 << 20
)

type Source struct {
	ID   uint64
	Path string
	Pix  *image.NRGBA
	W, H int
}

var (
	sourceIDBase    uint64
	sourceIDCounter atomic.Uint64
)

func init() {
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		now := uint64(time.Now().UnixNano())
		binary.LittleEndian.PutUint64(seed[:8], now)
		binary.LittleEndian.PutUint64(seed[8:], now^0x9e3779b97f4a7c15)
	}
	r := randv2.New(randv2.NewPCG(
		binary.LittleEndian.Uint64(seed[:8]),
		binary.LittleEndian.Uint64(seed[8:]),
	))
	// Kitty image ids share one global space with every other program on the same terminal, so a fixed per-process base would collide with e.g. an earlier `kitty icat`.
	sourceIDBase = r.Uint64() & 0xFFFFFF
}

func nextSourceID() uint64 {
	return sourceIDBase + sourceIDCounter.Add(1)
}

func Load(path string) (*Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("image: read %s: %w", path, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxImageFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("image: read %s: %w", path, err)
	}
	if len(data) > maxImageFileBytes {
		return nil, fmt.Errorf("image: %s exceeds maximum %d bytes", path, maxImageFileBytes)
	}
	return decode(data, path)
}

func decode(data []byte, name string) (*Source, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("image: decode config %s: %w", name, err)
	}
	// Reject before decoding: a hostile file can declare gigapixel dimensions in a few header bytes and exhaust memory on decode.
	if cfg.Width > maxImageDimension || cfg.Height > maxImageDimension {
		return nil, fmt.Errorf("image: %s dimensions %dx%d exceed maximum %d", name, cfg.Width, cfg.Height, maxImageDimension)
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxImagePixels {
		return nil, fmt.Errorf("image: %s pixel count %d exceeds maximum %d", name, cfg.Width*cfg.Height, maxImagePixels)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("image: decode %s: %w", name, err)
	}
	bounds := img.Bounds()
	// Kitty f=32 is straight alpha: NRGBA, not the premultiplied RGBA.
	rgba := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, bounds.Min, draw.Src)
	return &Source{
		ID:   nextSourceID(),
		Path: name,
		Pix:  rgba,
		W:    bounds.Dx(),
		H:    bounds.Dy(),
	}, nil
}
