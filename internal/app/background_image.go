package app

import (
	"os"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term/kittygfx"
)

// backgroundImageState tracks what has actually been placed on the
// terminal, so a settings change only re-transmits when the image itself
// (path or dim) changed, versus a resize which always needs to re-crop and
// re-transmit since the cover-fit depends on the terminal's pixel size.
type backgroundImageState struct {
	active bool
	path   string
	dim    int
}

// applyBackgroundImage reacts to a settings change, transmitting and placing
// (or clearing) the Kitty-graphics-protocol background image as needed. It
// never blocks ApplySettings on failure: an unsupported terminal, a missing
// file, or a decode error simply leaves the background image inactive.
func (a *App) applyBackgroundImage(prev, cur config.EditorSettings) {
	if prev.BackgroundImage == cur.BackgroundImage && prev.BackgroundImageDim == cur.BackgroundImageDim {
		return
	}
	if a.bgImage.active {
		a.deleteBackgroundImage()
	}
	if cur.BackgroundImage == "" {
		return
	}
	if err := a.transmitAndPlaceBackgroundImage(cur.BackgroundImage, cur.BackgroundImageDim); err != nil {
		a.StatusError("Background image: " + err.Error())
	}
}

// reapplyBackgroundImagePlacement re-crops and re-transmits the background
// image for the terminal's current pixel size. Called on resize: the
// cover-fit crop depends on the window's pixel dimensions, not just its
// column/row count, so a resize cannot simply re-place cached pixel data.
func (a *App) reapplyBackgroundImagePlacement() {
	if !a.bgImage.active {
		return
	}
	if err := a.transmitAndPlaceBackgroundImage(a.bgImage.path, a.bgImage.dim); err != nil {
		a.StatusError("Background image: " + err.Error())
	}
}

func (a *App) transmitAndPlaceBackgroundImage(path string, dimPercent int) error {
	if a.Screen == nil {
		return nil
	}
	tty, ok := a.Screen.Tty()
	if !ok {
		return nil
	}
	if !kittygfx.DetectSupport(os.Getenv) {
		return nil
	}

	var pxW, pxH int
	if ws, err := tty.WindowSize(); err == nil {
		pxW, pxH = ws.PixelWidth, ws.PixelHeight
	}

	png, err := kittygfx.EncodePNG(path, float64(dimPercent)/100, pxW, pxH)
	if err != nil {
		return err
	}
	if err := kittygfx.Transmit(tty, kittygfx.ImageID, png); err != nil {
		return err
	}
	cols, rows := a.Screen.Size()
	if err := kittygfx.Place(tty, kittygfx.ImageID, cols, rows); err != nil {
		return err
	}
	a.bgImage = backgroundImageState{active: true, path: path, dim: dimPercent}
	return nil
}

func (a *App) deleteBackgroundImage() {
	a.bgImage = backgroundImageState{}
	if a.Screen == nil {
		return
	}
	tty, ok := a.Screen.Tty()
	if !ok {
		return
	}
	_ = kittygfx.Delete(tty, kittygfx.ImageID)
}

// SeedBackgroundImage transmits and places the configured background image,
// if any, once at startup. ApplySettings is not invoked during App.Init, so
// this is the only path that draws the image before the first settings
// change.
func (a *App) SeedBackgroundImage() {
	a.applyBackgroundImage(config.EditorSettings{}, a.Settings.Editor)
}

// CloseBackgroundImage deletes the active placement, if any. Called on
// shutdown, before the screen is finalized.
func (a *App) CloseBackgroundImage() {
	if a.bgImage.active {
		a.deleteBackgroundImage()
	}
}
