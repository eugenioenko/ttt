package ui

import (
	"path/filepath"
	"sort"
	"strings"
)

func fileDetail(f string) string {
	dir := filepath.Dir(f)
	if dir == "." {
		return ""
	}
	return dir
}

func fuzzyFilterFiles(files []PaletteFile, query string, maxResults int) []PaletteItem {
	query = strings.Join(strings.Fields(query), "")
	if query == "" {
		items := make([]PaletteItem, 0, min(len(files), maxResults))
		for _, f := range files {
			items = append(items, PaletteItem{
				Label:  filepath.Base(f.Rel),
				Detail: fileDetail(f.Rel),
				ID:     f.Abs,
			})
			if len(items) >= maxResults {
				break
			}
		}
		return items
	}

	type scored struct {
		item  PaletteItem
		score int
	}
	var matches []scored
	for _, f := range files {
		if ok, score := fuzzyMatch(query, f.Rel); ok {
			matches = append(matches, scored{
				item: PaletteItem{
					Label:  filepath.Base(f.Rel),
					Detail: fileDetail(f.Rel),
					ID:     f.Abs,
				},
				score: score,
			})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})
	items := make([]PaletteItem, 0, min(len(matches), maxResults))
	for _, m := range matches {
		items = append(items, m.item)
		if len(items) >= maxResults {
			break
		}
	}
	return items
}
