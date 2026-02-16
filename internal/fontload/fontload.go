package fontload

import (
	"os"

	"golang.org/x/image/font/sfnt"
)

// ScalableFont is a parsed scalable font with original bytes and SFNT view.
type ScalableFont struct {
	Fontname string
	//Filepath string
	Binary []byte
	SFNT   *sfnt.Font
}

// LoadOpenTypeFont loads an OpenType font (TTF or OTF) from a file.
func LoadOpenTypeFont(fontfile string) (*ScalableFont, error) {
	bytez, err := os.ReadFile(fontfile)
	if err != nil {
		return nil, err
	}
	f, err := ParseOpenTypeFont(bytez)
	if err != nil {
		return nil, err
	}
	//f.Filepath = fontfile
	return f, nil
}

// ParseOpenTypeFont loads an OpenType font (TTF or OTF) from memory.
func ParseOpenTypeFont(fbytes []byte) (f *ScalableFont, err error) {
	f = &ScalableFont{Binary: fbytes}
	f.SFNT, err = sfnt.Parse(f.Binary)
	if err != nil {
		return nil, err
	}
	f.Fontname, err = f.SFNT.Name(nil, sfnt.NameIDFull)
	return f, nil
}
