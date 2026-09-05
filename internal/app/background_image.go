package app

import (
	"os"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term/kittygfx"
)

// backgroundImageState tracks what has actually been transmitted/placed on
// the terminal, so a resize can re-place without re-transmitting pixel data
// and a settings change only re-transmits when the image itself changed.
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
	if a.Screen == nil {
		return
	}
	tty, ok := a.Screen.Tty()
	if !ok {
		return
	}
	if !kittygfx.DetectSupport(os.Getenv) {
		return
	}

	if a.bgImage.active {
		_ = kittygfx.Delete(tty, kittygfx.ImageID)
		a.bgImage = backgroundImageState{}
	}

	if cur.BackgroundImage == "" {
		return
	}

	png, err := kittygfx.EncodePNG(cur.BackgroundImage, float64(cur.BackgroundImageDim)/100)
	if err != nil {
		a.StatusError("Background image: " + err.Error())
		return
	}
	if err := kittygfx.Transmit(tty, kittygfx.ImageID, png); err != nil {
		a.StatusError("Background image: " + err.Error())
		return
	}
	cols, rows := a.Screen.Size()
	if err := kittygfx.Place(tty, kittygfx.ImageID, cols, rows); err != nil {
		a.StatusError("Background image: " + err.Error())
		return
	}
	a.bgImage = backgroundImageState{active: true, path: cur.BackgroundImage, dim: cur.BackgroundImageDim}
}

// reapplyBackgroundImagePlacement re-places the already-transmitted
// background image at the terminal's current size, without re-sending pixel
// data. Called on resize.
func (a *App) reapplyBackgroundImagePlacement() {
	if !a.bgImage.active || a.Screen == nil {
		return
	}
	tty, ok := a.Screen.Tty()
	if !ok {
		return
	}
	cols, rows := a.Screen.Size()
	_ = kittygfx.Place(tty, kittygfx.ImageID, cols, rows)
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
	if !a.bgImage.active || a.Screen == nil {
		return
	}
	tty, ok := a.Screen.Tty()
	if !ok {
		return
	}
	_ = kittygfx.Delete(tty, kittygfx.ImageID)
	a.bgImage = backgroundImageState{}
}
