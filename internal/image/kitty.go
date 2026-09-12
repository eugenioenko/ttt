package image

import (
	"encoding/base64"
	"fmt"
	"io"
)

// Base64 payload bytes per Kitty chunk; the terminal reassembles while m=1 and acts on the chunk carrying m=0.
const kittyChunkSize = 4096

func writeAPC(w io.Writer, params, payload string) error {
	_, err := fmt.Fprintf(w, "\x1b_G%s;%s\x1b\\", params, payload)
	if err != nil {
		return fmt.Errorf("image: write kitty sequence: %w", err)
	}
	return nil
}

func Transmit(w io.Writer, s *Source) error {
	if s == nil || s.Pix == nil {
		return fmt.Errorf("image: transmit nil source")
	}
	raw := s.Pix.Pix
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(raw)))
	base64.StdEncoding.Encode(encoded, raw)
	chunks := len(encoded) / kittyChunkSize
	if len(encoded)%kittyChunkSize != 0 {
		chunks++
	}
	if chunks == 0 {
		chunks = 1
	}
	for i := range chunks {
		lo := i * kittyChunkSize
		hi := min(lo+kittyChunkSize, len(encoded))
		m := 0
		if i < chunks-1 {
			m = 1
		}
		var params string
		if i == 0 {
			// a=t transmits only; a=T would also display at the cursor, which tcell parks at the bottom-right, scrolling the whole screen.
			params = fmt.Sprintf("a=t,q=2,f=32,t=d,s=%d,v=%d,i=%d,m=%d", s.W, s.H, s.ID, m)
		} else {
			params = fmt.Sprintf("m=%d", m)
		}
		if err := writeAPC(w, params, string(encoded[lo:hi])); err != nil {
			return err
		}
	}
	return nil
}

func Place(w io.Writer, p Placement) error {
	if _, err := fmt.Fprintf(w, "\x1b[%d;%dH", p.Cell.Y+1, p.Cell.X+1); err != nil {
		return fmt.Errorf("image: write cursor position: %w", err)
	}
	params := fmt.Sprintf("a=p,q=2,i=%d,p=%d,C=1,x=%d,y=%d,w=%d,h=%d,c=%d,r=%d,z=%d",
		p.SourceID, p.PlacementID,
		p.Src.Min.X, p.Src.Min.Y, p.Src.Dx(), p.Src.Dy(),
		p.Cell.W, p.Cell.H, p.Z)
	return writeAPC(w, params, "")
}

func DeletePlacement(w io.Writer, imageID uint64, placementID uint32) error {
	return writeAPC(w, fmt.Sprintf("a=d,q=2,d=i,i=%d,p=%d", imageID, placementID), "")
}

func DeleteImage(w io.Writer, imageID uint64) error {
	return writeAPC(w, fmt.Sprintf("a=d,q=2,d=I,i=%d", imageID), "")
}
