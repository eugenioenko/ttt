package widgets

import (
	"fmt"
	stdimage "image"
	"strings"

	"github.com/eugenioenko/ttt/internal/image"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

const (
	ImageFitContain = "contain"
	ImageFitNone    = "none"
)

type ImageConfig struct {
	Data   string
	Fit    string
	Alt    string
	Height int

	Protocol     image.Protocol
	CellW, CellH int
	// Redraw is called from another goroutine when a decode finishes. A large page takes hundreds of
	// milliseconds to decode, so with Redraw set decoding leaves the UI goroutine; nil decodes inline.
	Redraw func()
}

type decodeResult struct {
	src *image.Source
	err error
}

type ImageWidget struct {
	BaseWidget
	Config ImageConfig

	src         *image.Source
	err         string
	info        string
	pending     chan decodeResult
	pendingData string
	release     func(id uint64)
}

var decodeImage = image.Decode

func NewImageWidget(config ImageConfig) *ImageWidget {
	w := &ImageWidget{}
	w.SetConfig(config)
	return w
}

// SetConfig keeps the decoded source while Data is unchanged, so a re-render neither decodes nor re-transmits.
// Data is a string so that passing the same plugin string again compares by pointer instead of by content.
func (w *ImageWidget) SetConfig(config ImageConfig) {
	if config.Data != w.Config.Data {
		w.dropSource()
		w.err = ""
		w.info = ""
	}
	w.Config = config
}

func (w *ImageWidget) Height() int {
	if w.Config.Height > 0 {
		return w.Config.Height + w.BoxOverheadH()
	}
	return 0
}

func (w *ImageWidget) Width() int { return 0 }

func (w *ImageWidget) HandleEvent(tcell.Event) EventResult { return EventIgnored }

func (w *ImageWidget) Close() { w.dropSource() }

func (w *ImageWidget) dropSource() {
	if w.src != nil && w.release != nil {
		w.release(w.src.ID)
	}
	w.src = nil
}

func (w *ImageWidget) Render(surface Surface) {
	inner := w.RenderBox(surface)
	bw, bh := inner.Size()
	if bw <= 0 || bh <= 0 {
		return
	}
	if w.place(inner, bw, bh) {
		return
	}
	inner.DrawText(0, 0, w.fallbackText(), bw, term.StyleMuted)
}

// Place renders like Render but leaves the fallback to the caller when it returns false.
func (w *ImageWidget) Place(surface Surface) bool {
	inner := w.RenderBox(surface)
	bw, bh := inner.Size()
	return bw > 0 && bh > 0 && w.place(inner, bw, bh)
}

func (w *ImageWidget) Err() string { return w.err }

func (w *ImageWidget) Decoding() bool { return w.pending != nil }

func (w *ImageWidget) place(surface Surface, bw, bh int) bool {
	placer, ok := surface.(ImagePlacer)
	if !ok {
		return false
	}
	w.release = placer.ImageReleaser()
	cfg := w.Config
	if cfg.Protocol != image.Kitty || cfg.CellW <= 0 || cfg.CellH <= 0 {
		return false
	}
	src := w.decoded()
	if src == nil {
		return false
	}
	if cfg.Fit == ImageFitNone {
		cw := (src.W + cfg.CellW - 1) / cfg.CellW
		ch := (src.H + cfg.CellH - 1) / cfg.CellH
		placer.PlaceImage(0, 0, cw, ch, src)
		return true
	}
	cw, ch, ok := image.FitCells(src.W, src.H, bw, bh, cfg.CellW, cfg.CellH)
	if !ok {
		return false
	}
	placer.PlaceImage((bw-cw)/2, (bh-ch)/2, cw, ch, src)
	return true
}

// A rejected image is not retried until its content changes, so a broken image never costs per-frame decoding.
// At most one decode runs per widget: data that changes mid-decode waits for it, so paging fast cannot pile up
// full-size decodes in memory.
func (w *ImageWidget) decoded() *image.Source {
	if w.pending != nil {
		select {
		case r := <-w.pending:
			w.pending = nil
			if w.pendingData == w.Config.Data {
				w.setDecoded(r.src, r.err)
			}
		default:
			return nil
		}
	}
	if w.src != nil || w.err != "" {
		return w.src
	}
	data := w.Config.Data
	if w.Config.Redraw == nil {
		w.setDecoded(decodeImage([]byte(data), "data"))
		return w.src
	}
	ch := make(chan decodeResult, 1)
	w.pending, w.pendingData = ch, data
	redraw := w.Config.Redraw
	go func() {
		src, err := decodeImage([]byte(data), "data")
		ch <- decodeResult{src, err}
		redraw()
	}()
	// The result is taken on the frame Redraw triggers, never this one, so the pending state always renders once.
	return nil
}

func (w *ImageWidget) setDecoded(src *image.Source, err error) {
	if err != nil {
		w.err = err.Error()
		return
	}
	w.src = src
}

func (w *ImageWidget) fallbackText() string {
	if w.Config.Alt != "" {
		return w.Config.Alt
	}
	if w.err != "" {
		return w.err
	}
	if w.info == "" {
		cfg, format, err := stdimage.DecodeConfig(strings.NewReader(w.Config.Data))
		if err != nil {
			w.info = "Image: " + err.Error()
		} else {
			w.info = fmt.Sprintf("%s image %dx%d", strings.ToUpper(format), cfg.Width, cfg.Height)
		}
	}
	return w.info
}
