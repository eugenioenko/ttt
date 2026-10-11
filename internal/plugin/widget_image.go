package plugin

import (
	"github.com/eugenioenko/ttt/internal/image"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type GraphicsInfo struct {
	Protocol     image.Protocol
	CellW, CellH int
}

func (p *Plugin) graphics() GraphicsInfo {
	if p.Graphics == nil {
		return GraphicsInfo{}
	}
	return p.Graphics()
}

func imageConfig(desc WidgetDesc, p *Plugin) widgets.ImageConfig {
	g := p.graphics()
	return widgets.ImageConfig{
		Data:     desc.ImageData,
		Fit:      desc.ImageFit,
		Alt:      desc.ImageAlt,
		Height:   desc.ImageHeight,
		Protocol: g.Protocol,
		CellW:    g.CellW,
		CellH:    g.CellH,
		Redraw:   p.RequestRedraw,
	}
}

// A new image reuses a dropped image with the same data, since recreating it would decode and transmit it
// again. Dropped widgets never render again, so their images are freed here.
func reconcileSlots(old []widgets.Widget, descs []WidgetDesc, p *Plugin, same func(i int) bool) []widgets.Widget {
	out := make([]widgets.Widget, len(descs))
	complete := len(old) <= len(descs)
	for i, d := range descs {
		if same(i) {
			updateWidget(old[i], d, p)
			out[i] = old[i]
		} else {
			complete = false
		}
	}
	if complete {
		return out
	}
	dropped := make([]bool, len(old))
	for j, w := range old {
		dropped[j] = j >= len(out) || out[j] != w
	}
	for i, d := range descs {
		if out[i] != nil {
			continue
		}
		if d.Kind == WidgetImage {
			for j, w := range old {
				if iw, ok := w.(*widgets.ImageWidget); ok && dropped[j] && iw.Config.Data == d.ImageData {
					dropped[j] = false
					updateWidget(iw, d, p)
					out[i] = iw
					break
				}
			}
		}
		if out[i] == nil {
			out[i] = createWidget(d, p)
		}
	}
	for j, w := range old {
		if dropped[j] {
			closeImages(w)
		}
	}
	return out
}

func closeImages(w widgets.Widget) {
	switch v := w.(type) {
	case nil:
		return
	case *widgets.ImageWidget:
		v.Close()
	case widgets.ContainerWidget:
		for _, child := range v.WidgetChildren() {
			closeImages(child)
		}
	}
}
