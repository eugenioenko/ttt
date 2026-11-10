package config

type EditorViewState struct {
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	Col     int    `json:"col,omitempty"`
	TopLine int    `json:"topLine,omitempty"`
	LeftCol int    `json:"leftCol,omitempty"`
	Pinned  bool   `json:"pinned,omitempty"`
	Active  bool   `json:"active,omitempty"`
}

type EditorLayoutState struct {
	Direction string             `json:"direction,omitempty"`
	Ratio     float64            `json:"ratio,omitempty"`
	First     *EditorLayoutState `json:"first,omitempty"`
	Second    *EditorLayoutState `json:"second,omitempty"`
	Views     []EditorViewState  `json:"views,omitempty"`
	Focused   bool               `json:"focused,omitempty"`
}
