package term

import "unique"

type hyperlinkData struct {
	url, id string
}

// Canonical handles keep cells small and comparable without retaining expired links globally.
type Hyperlink struct {
	value unique.Handle[hyperlinkData]
}

func NewHyperlink(url, id string) Hyperlink {
	if url == "" {
		return Hyperlink{}
	}
	return Hyperlink{value: unique.Make(hyperlinkData{url: url, id: id})}
}

func (h Hyperlink) URL() string {
	if h == (Hyperlink{}) {
		return ""
	}
	return h.value.Value().url
}

func (h Hyperlink) ID() string {
	if h == (Hyperlink{}) {
		return ""
	}
	return h.value.Value().id
}
