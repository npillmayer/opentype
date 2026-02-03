package otlayout

import "github.com/npillmayer/opentype/ot"

// ListAll collects all items from a NavList.
func ListAll(l ot.NavList) []ot.NavLocation {
	if l == nil || l.Len() == 0 {
		return nil
	}
	r := make([]ot.NavLocation, 0, l.Len())
	for _, loc := range l.Range() {
		r = append(r, loc)
	}
	return r
}
