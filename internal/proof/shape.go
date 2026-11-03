package proof

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otarabic"
	"github.com/npillmayer/opentype/otshape/otcore"
	"github.com/npillmayer/opentype/otshape/othebrew"
)

type IO struct {
	Source otshape.RuneSource
	Sink   otshape.GlyphSink
}

func Shape(io IO, params otshape.Params, bufOpts otshape.BufferOptions) error {
	engines := []otshape.ShapingEngine{
		otcore.New(),
		otarabic.New(),
		othebrew.New(),
	}
	shaper := otshape.NewShaper(engines...)
	err := shaper.Shape(params, io.Source, io.Sink, bufOpts)
	return err
}

var noFeatures = otshape.FeatureRange{}

// We try to follow Harfbuzz's `hb-shape` features parameter syntax, which is
// unfortunately not well documented.
//
// Disable ligatures:      --features="-liga"
// or                      --features="liga=0"
//
// Ranges use original Unicode scalar indexes, inclusive start/exclusive end.
func ParseFeatureItem(item string) (otshape.FeatureRange, error) {
	if item = strings.TrimSpace(item); item == "" {
		return noFeatures, errors.New("empty feature entry in --features")
	}
	on := true
	if strings.HasPrefix(item, "+") {
		item = item[1:]
	} else if strings.HasPrefix(item, "-") {
		item, on = item[1:], false
	}
	tagPart, value, arg := item, "", 1
	hasEqual := false
	if tagPart, value, hasEqual = strings.Cut(item, "="); hasEqual {
		if value == "" {
			return noFeatures, fmt.Errorf("empty feature value in %q", item)
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return noFeatures, fmt.Errorf("invalid feature value in %q: %w", item, err)
		}
		arg = n
		on = n != 0
	}
	tagPart = strings.TrimSpace(tagPart)
	start, end := 0, 0
	if tag, bounds, found := strings.Cut(tagPart, "["); found {
		if !strings.HasSuffix(bounds, "]") {
			return noFeatures, fmt.Errorf("invalid feature range in %q", item)
		}
		bounds = strings.TrimSuffix(bounds, "]")
		left, right, colon := strings.Cut(bounds, ":")
		if !colon {
			return noFeatures, fmt.Errorf("range must be [start:end] in %q", item)
		}
		var err error
		if left != "" {
			start, err = strconv.Atoi(left)
			if err != nil || start < 0 {
				return noFeatures, fmt.Errorf("invalid range start in %q", item)
			}
		}
		if right != "" {
			end, err = strconv.Atoi(right)
			if err != nil || end <= start {
				return noFeatures, fmt.Errorf("range end must exceed start in %q", item)
			}
		}
		tagPart = tag
	}
	if len(tagPart) != 4 {
		return noFeatures, fmt.Errorf("feature tag %q is not 4 characters", tagPart)
	}
	return otshape.FeatureRange{
		Feature: ot.T(tagPart),
		Arg:     arg,
		On:      on,
		Start:   start, End: end,
	}, nil
}

func ParseCodepoints(spec string) ([]rune, error) {
	parts := SplitCSVSpace(spec)
	out := make([]rune, 0, len(parts))
	for _, p := range parts {
		r, err := ParseCodepointToken(p)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func ParseCodepointToken(token string) (rune, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, errors.New("empty codepoint token")
	}
	hex := token
	switch {
	case strings.HasPrefix(hex, "U+"), strings.HasPrefix(hex, "u+"):
		hex = hex[2:]
	case strings.HasPrefix(hex, "0x"), strings.HasPrefix(hex, "0X"):
		hex = hex[2:]
	}
	u, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid codepoint %q: %w", token, err)
	}
	if u > utf8.MaxRune || !utf8.ValidRune(rune(u)) {
		return 0, fmt.Errorf("invalid Unicode scalar %q", token)
	}
	return rune(u), nil
}

func SplitCSVSpace(spec string) []string {
	return strings.FieldsFunc(spec, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
}

// ---Formatting the Output ---------------------------------------------

type Collector struct {
	Glyphs []otshape.GlyphRecord
}

func (c *Collector) WriteGlyph(g otshape.GlyphRecord) error {
	c.Glyphs = append(c.Glyphs, g)
	return nil
}

type cancelSource struct {
	ctx    context.Context
	source otshape.RuneSource
}

func (s cancelSource) ReadRune() (rune, int, error) {
	if err := s.ctx.Err(); err != nil {
		return 0, 0, err
	}
	return s.source.ReadRune()
}

type cancelSink struct {
	ctx  context.Context
	sink otshape.GlyphSink
}

func (s cancelSink) WriteGlyph(g otshape.GlyphRecord) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	return s.sink.WriteGlyph(g)
}
