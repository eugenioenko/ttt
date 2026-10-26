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

	wrap  bool
	lastW int
}

func NewKeyValueListWidget(entries []KeyValueEntry) *KeyValueListWidget {
	return &KeyValueListWidget{Entries: entries}
}

// WrappingKeyValueListWidget wraps long values instead of clipping them. It is
// a separate type because HeightForWidth stops a scroll view from scrolling
// its content horizontally.
type WrappingKeyValueListWidget struct {
	*KeyValueListWidget
}

func NewWrappingKeyValueListWidget(entries []KeyValueEntry) *WrappingKeyValueListWidget {
	return &WrappingKeyValueListWidget{&KeyValueListWidget{Entries: entries, wrap: true}}
}

func (w *WrappingKeyValueListWidget) HeightForWidth(width int) int {
	return w.rowCount(width-w.BoxOverheadW()) + w.BoxOverheadH()
}

func (kv *KeyValueListWidget) Height() int {
	if kv.wrap && kv.lastW > 0 {
		return kv.rowCount(kv.lastW) + kv.BoxOverheadH()
	}
	return len(kv.Entries) + kv.BoxOverheadH()
}

func (kv *KeyValueListWidget) Width() int { return 0 }

func (kv *KeyValueListWidget) ScrollSize() (int, int) {
	if len(kv.Entries) == 0 {
		return 0, 0
	}
	if kv.wrap && kv.lastW > 0 {
		return kv.lastW, kv.rowCount(kv.lastW)
	}
	return kv.unwrappedWidth(), len(kv.Entries)
}

// NaturalWidth is the width at which every entry fits on one row.
func (kv *KeyValueListWidget) NaturalWidth() int {
	return kv.unwrappedWidth() + kv.BoxOverheadW()
}

func (kv *KeyValueListWidget) unwrappedWidth() int {
	valueW := 0
	for _, e := range kv.Entries {
		valueW = max(valueW, textwidth.String(e.Value))
	}
	return kv.keyColWidth() + 2 + valueW
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

func (kv *KeyValueListWidget) layoutKeyColW(w int) int {
	return min(kv.keyColWidth(), w/2)
}

func (kv *KeyValueListWidget) valueRows(value string, w int) []string {
	valW := w - kv.layoutKeyColW(w) - 2
	if !kv.wrap || valW < 1 {
		return []string{value}
	}
	runes := []rune(value)
	rows := wrapInputRows(runes, valW)
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = string(runes[r.start:r.end])
	}
	return out
}

func (kv *KeyValueListWidget) rowCount(w int) int {
	n := 0
	for _, e := range kv.Entries {
		n += len(kv.valueRows(e.Value, w))
	}
	return n
}

func (kv *KeyValueListWidget) Render(surface Surface) {
	surface = kv.RenderBox(surface)
	w, h := surface.Size()
	if w <= 0 || h <= 0 {
		return
	}
	kv.lastW = w

	keyColW := kv.layoutKeyColW(w)

	keyStyle, valStyle := term.StylePaletteItem, term.StyleMuted
	if kv.InvertStyles {
		keyStyle, valStyle = valStyle, keyStyle
	}

	y := 0
	for _, entry := range kv.Entries {
		if y >= h {
			break
		}
		kx := max(keyColW-textwidth.String(entry.Key), 0)
		surface.DrawText(kx, y, entry.Key, keyColW, keyStyle)

		valX := keyColW + 2
		for _, row := range kv.valueRows(entry.Value, w) {
			if y >= h {
				break
			}
			surface.DrawText(valX, y, row, w, valStyle)
			y++
		}
	}
}

func (kv *KeyValueListWidget) HandleEvent(ev tcell.Event) EventResult {
	return EventIgnored
}
