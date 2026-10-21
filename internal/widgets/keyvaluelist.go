package widgets

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

type KeyValueEntry struct {
	Key   string
	Value string
}

type KeyValueListWidget struct {
	BaseWidget
	Entries      []KeyValueEntry
	InvertStyles bool
}

func NewKeyValueListWidget(entries []KeyValueEntry) *KeyValueListWidget {
	return &KeyValueListWidget{Entries: entries}
}

func (kv *KeyValueListWidget) Height() int { return len(kv.Entries) + kv.BoxOverheadH() }
func (kv *KeyValueListWidget) Width() int  { return 0 }

func (kv *KeyValueListWidget) ScrollSize() (int, int) {
	w := 0
	for _, e := range kv.Entries {
		row := textwidth.String(e.Key) + 4 + textwidth.String(e.Value)
		if row > w {
			w = row
		}
	}
	return w, len(kv.Entries)
}

func (kv *KeyValueListWidget) keyColWidth() int {
	w := 0
	for _, e := range kv.Entries {
		if n := textwidth.String(e.Key); n > w {
			w = n
		}
	}
	return w + 2
}

func (kv *KeyValueListWidget) Render(surface Surface) {
	surface = kv.RenderBox(surface)
	w, h := surface.Size()
	if w <= 0 || h <= 0 {
		return
	}

	keyColW := kv.keyColWidth()
	if keyColW > w/2 {
		keyColW = w / 2
	}

	keyStyle, valStyle := term.StylePaletteItem, term.StyleMuted
	if kv.InvertStyles {
		keyStyle, valStyle = valStyle, keyStyle
	}

	for y := range h {
		if y >= len(kv.Entries) {
			break
		}
		entry := kv.Entries[y]

		kx := keyColW - textwidth.String(entry.Key)
		if kx < 0 {
			kx = 0
		}
		surface.DrawText(kx, y, entry.Key, keyColW, keyStyle)

		valX := keyColW + 2
		surface.DrawText(valX, y, entry.Value, w, valStyle)
	}
}

func (kv *KeyValueListWidget) HandleEvent(ev tcell.Event) EventResult {
	return EventIgnored
}
