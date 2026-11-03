package proof

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"math"
	"runtime/debug"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

// Proof cases describe inputs, not expected results. Keep this schema separate
// from test oracle formats and from the internal shaper's buffer representation.
type Case struct {
	Version    int     `json:"version"`
	Font       string  `json:"font"`
	FontSHA256 string  `json:"font_sha256,omitempty"`
	Text       *string `json:"text,omitempty"`
	TextFile   string  `json:"text_file,omitempty"`
	Settings
	Features []string       `json:"features,omitempty"`
	TestFont bool           `json:"testfont,omitempty"`
	Buffer   Buffer         `json:"buffer"`
	Render   RenderSettings `json:"render"`
	Variants []Variant      `json:"variants"`
}

type Variant struct {
	Label    string   `json:"label"`
	Features []string `json:"features,omitempty"`
}

type Buffer struct {
	Flush string `json:"flush,omitempty"`
	High  int    `json:"high_watermark,omitempty"`
	Low   int    `json:"low_watermark,omitempty"`
	Max   int    `json:"max_buffer,omitempty"`
}

func (b Buffer) options() (otshape.BufferOptions, error) {
	f := otshape.FlushOnRunBoundary
	switch b.Flush {
	case "", "run":
	case "cluster":
		f = otshape.FlushOnClusterBoundary
	default:
		return otshape.BufferOptions{}, fmt.Errorf("invalid buffer.flush %q", b.Flush)
	}
	return otshape.BufferOptions{FlushBoundary: f, HighWatermark: b.High, LowWatermark: b.Low, MaxBuffer: b.Max}, nil
}

type RenderSettings struct {
	PPEM        int  `json:"ppem,omitempty"`
	Diagnostics bool `json:"diagnostics,omitempty"`
	Boxes       bool `json:"show_bboxes,omitempty"`
	Baseline    bool `json:"show_baseline,omitempty"`
	Origins     bool `json:"show_origins,omitempty"`
	Labels      bool `json:"show_labels,omitempty"`
	Pens        bool `json:"show_pens,omitempty"`
	Advances    bool `json:"show_advances,omitempty"`
	Clusters    bool `json:"color_clusters,omitempty"`
}

func (r RenderSettings) options(dir bidi.Direction) Options {
	p := r.PPEM
	if p == 0 {
		p = 96
	}
	o := Options{PPEM: p, Direction: dir, ShowBoxes: r.Boxes, ShowBaseline: r.Baseline, ShowOrigins: r.Origins, ShowLabels: r.Labels, ShowPens: r.Pens, ShowAdvances: r.Advances, ColorClusters: r.Clusters}
	if r.Diagnostics {
		o.Diagnostics()
	}
	return o
}

type Result struct {
	Version            int         `json:"version"`
	Build              proofBuild  `json:"build"`
	Case               Case        `json:"case"`
	Units              string      `json:"units"`
	ClusterCoordinates string      `json:"cluster_coordinates"`
	GlyphOrder         string      `json:"glyph_order"`
	Notes              []string    `json:"notes,omitempty"`
	Rows               []RowResult `json:"rows"`
}

type proofBuild struct {
	GoVersion string `json:"go_version,omitempty"`
	Module    string `json:"module,omitempty"`
	Version   string `json:"version,omitempty"`
	Revision  string `json:"revision,omitempty"`
	Modified  string `json:"modified,omitempty"`
}

func currentProofBuild() proofBuild {
	var b proofBuild
	if info, ok := debug.ReadBuildInfo(); ok {
		b.GoVersion, b.Module, b.Version = info.GoVersion, info.Main.Path, info.Main.Version
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				b.Revision = setting.Value
			case "vcs.modified":
				b.Modified = setting.Value
			}
		}
	}
	return b
}

type RowResult struct {
	Label    string      `json:"label"`
	Features []string    `json:"features"`
	Glyphs   []GlyphData `json:"glyphs"`
}

type GlyphData struct {
	Index       int           `json:"index"`
	VisualIndex int           `json:"visual_index"`
	GID         ot.GlyphIndex `json:"gid"`
	Cluster     uint32        `json:"cluster"`
	XAdvance    int32         `json:"x_advance"`
	YAdvance    int32         `json:"y_advance"`
	XOffset     int32         `json:"x_offset"`
	YOffset     int32         `json:"y_offset"`
	PenX        int64         `json:"pen_x"`
	PenY        int64         `json:"pen_y"`
	OriginX     int64         `json:"origin_x"`
	OriginY     int64         `json:"origin_y"`
}

func glyphData(glyphs []otshape.GlyphRecord, dir bidi.Direction) []GlyphData {
	out := make([]GlyphData, len(glyphs))
	var x, y int64
	for vi := range glyphs {
		i := vi
		if dir == bidi.RightToLeft {
			i = len(glyphs) - 1 - vi
		}
		g := glyphs[i]
		p := g.Pos
		out[i] = GlyphData{Index: i, VisualIndex: vi, GID: g.GID, Cluster: g.Cluster, XAdvance: p.XAdvance, YAdvance: p.YAdvance, XOffset: p.XOffset, YOffset: p.YOffset, PenX: x, PenY: y, OriginX: x + int64(p.XOffset), OriginY: y + int64(p.YOffset)}
		x += int64(p.XAdvance)
		y += int64(p.YAdvance)
	}
	return out
}

func DecodeCase(data []byte) (Case, error) {
	var c Case
	if !utf8.Valid(data) {
		return c, fmt.Errorf("case is not valid UTF-8")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("invalid proof case: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return c, fmt.Errorf("case must contain exactly one JSON object")
	}
	return c, nil
}
func NormalizeCase(c Case) (Case, error) {
	var err error
	if c.Version != 1 {
		return c, fmt.Errorf("unsupported proof-case version %d (expected 1)", c.Version)
	}
	if c.Font == "" {
		return c, fmt.Errorf("case requires font")
	}
	if (c.Text == nil) == (c.TextFile == "") {
		return c, fmt.Errorf("case requires exactly one of text or text_file")
	}
	if c.Text == nil {
		return c, fmt.Errorf("text_file must be loaded by the frontend")
	}
	if !utf8.ValidString(*c.Text) {
		return c, fmt.Errorf("case text is not valid UTF-8")
	}
	c.Settings, err = c.Settings.Resolve()
	if err != nil {
		return c, err
	}
	if len(c.Variants) == 0 || len(c.Variants) > 64 {
		return c, fmt.Errorf("case requires 1 to 64 variants")
	}
	labels := map[string]bool{}
	for _, v := range c.Variants {
		if strings.TrimSpace(v.Label) == "" || labels[v.Label] {
			return c, fmt.Errorf("variant labels must be nonempty and unique")
		}
		labels[v.Label] = true
	}
	if c.Render.PPEM == 0 {
		c.Render.PPEM = 96
	}
	if c.Buffer.Flush == "" {
		c.Buffer.Flush = "run"
	}
	return c, nil
}

func Execute(c Case, data []byte) (*Execution, error) {
	return ExecuteContext(context.Background(), c, data)
}

// ExecuteContext checks cancellation during ingestion, emission, and between
// variants. In-progress bounded font/lookup/raster operations are not preempted.
func ExecuteContext(ctx context.Context, c Case, data []byte) (*Execution, error) {
	var result Result
	c, err := NormalizeCase(c)
	if err != nil {
		return nil, err
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	if c.FontSHA256 != "" && !strings.EqualFold(c.FontSHA256, hash) {
		return nil, fmt.Errorf("font SHA-256 mismatch: got %s, expected %s", hash, c.FontSHA256)
	}
	c.FontSHA256 = hash
	var otf *ot.Font
	if c.TestFont {
		otf, err = ot.Parse(data, ot.IsTestfont)
	} else {
		otf, err = ot.Parse(data)
	}
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	sf, err := sfnt.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse outlines: %w", err)
	}
	script, _ := language.ParseScript(c.Script)
	lang, _ := language.Parse(c.Language)
	dir := bidi.LeftToRight
	if c.Direction == "rtl" {
		dir = bidi.RightToLeft
	}
	opts := c.Render.options(dir)
	buffer, err := c.Buffer.options()
	if err != nil {
		return nil, err
	}
	result = Result{Version: 1, Build: currentProofBuild(), Case: c, Units: "font_units_y_up", ClusterCoordinates: "original_unicode_scalar_indexes", GlyphOrder: "logical"}
	var rows []comparisonRow
	random := false
	for _, v := range c.Variants {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		specs := append(append([]string{}, c.Features...), v.Features...)
		features := make([]otshape.FeatureRange, 0, len(specs))
		for _, spec := range specs {
			f, err := ParseFeatureItem(spec)
			if err != nil {
				return nil, fmt.Errorf("variant %q: %w", v.Label, err)
			}
			features = append(features, f)
			random = random || (f.On && (f.Arg < 0 || f.Feature == ot.T("rand")))
		}
		sink := &Collector{}
		if err := Shape(IO{cancelSource{ctx, strings.NewReader(*c.Text)}, cancelSink{ctx, sink}}, otshape.Params{Font: otf, Script: script, Language: lang, Direction: dir, Features: features}, buffer); err != nil {
			return nil, fmt.Errorf("variant %q: %w", v.Label, err)
		}
		rows = append(rows, comparisonRow{label: v.Label, features: specs, glyphs: sink.Glyphs})
		result.Rows = append(result.Rows, RowResult{Label: v.Label, Features: specs, Glyphs: glyphData(sink.Glyphs, dir)})
	}
	if random {
		result.Notes = append(result.Notes, "Random alternate choices may differ between runs; the public shaper does not expose a fixed request seed.")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	img, geometry, err := renderComparison(sf, rows, opts)
	if err != nil {
		return nil, err
	}
	addClusterSpans(&geometry, result)
	return &Execution{Report: result, Image: img, Geometry: geometry}, nil
}

type comparisonRow struct {
	label    string
	features []string
	glyphs   []otshape.GlyphRecord
}

// Captions are deliberately ASCII: the tiny diagnostic font must not silently
// misrender Unicode labels. The actual sample always uses shaped font outlines.
func proofCaption(s string) string {
	quoted := strconv.QuoteToASCII(s)
	return quoted[1 : len(quoted)-1]
}

func comparisonGeometry(sf *sfnt.Font, rows []comparisonRow, opts Options) ([]proofLayout, proofLayout, error) {
	layouts := make([]proofLayout, 0, len(rows))
	var bounds proofLayout
	for _, r := range rows {
		l, err := layoutProof(sf, r.glyphs, opts)
		if err != nil {
			return nil, bounds, err
		}
		// Align each run's logical start: visual left edge of the advance for
		// LTR, visual right edge for RTL. Ink overhangs do not move the anchor.
		if opts.Direction == bidi.RightToLeft {
			shift := l.advanceX
			for i := range l.glyphs {
				g := &l.glyphs[i]
				g.x -= shift
				g.penX -= shift
				g.nextX -= shift
			}
			l.minX -= shift
			l.maxX -= shift
		}
		bounds.include(l.minX, l.minY)
		bounds.include(l.maxX, l.maxY)
		layouts = append(layouts, l)
	}
	return layouts, bounds, nil
}

func renderComparison(sf *sfnt.Font, rows []comparisonRow, opts Options) (*image.RGBA, Geometry, error) {
	var geometry Geometry
	if len(rows) == 0 || len(rows) > 64 {
		return nil, geometry, fmt.Errorf("comparison requires 1 to 64 rows")
	}
	layouts, bounds, err := comparisonGeometry(sf, rows, opts)
	if err != nil {
		return nil, geometry, err
	}
	const headerH = 40
	rowPlotH := math.Ceil(bounds.maxY-bounds.minY) + 2*proofPadding
	width := math.Ceil(bounds.maxX-bounds.minX) + 2*proofPadding
	height := 0.0
	captions := make([][]string, len(rows))
	legends := make([][]string, len(rows))
	for i, r := range rows {
		features := strings.Join(r.features, ", ")
		if features == "" {
			features = "font/engine defaults"
		}
		captions[i] = []string{proofCaption(r.label), "features: " + proofCaption(features)}
		if opts.ShowLabels {
			legends[i] = append(legends[i], "index: gid, scalar cluster, advance/offset in font units")
			for j, g := range r.glyphs {
				legends[i] = append(legends[i], glyphLegend(proofGlyph{record: g, logicalIndex: j}))
			}
		}
		for _, line := range append(append([]string{}, captions[i]...), legends[i]...) {
			width = math.Max(width, float64(font.MeasureString(basicfont.Face7x13, line))/64+2*proofPadding)
		}
		height += headerH + rowPlotH + float64(len(legends[i])*legendLineHeight)
	}
	if width > maxProofPixels || height > maxProofPixels || width*height > maxProofPixels {
		return nil, geometry, fmt.Errorf("comparison exceeds %d-pixel image budget; reduce scale, labels, or variants", maxProofPixels)
	}
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	draw.Draw(img, img.Bounds(), image.White, image.Point{}, draw.Src)
	tx := float64(proofPadding) - bounds.minX
	if opts.Direction == bidi.RightToLeft {
		tx = width - proofPadding - bounds.maxX
	}
	geometry.Width, geometry.Height = img.Bounds().Dx(), img.Bounds().Dy()
	top := 0.0
	for i, l := range layouts {
		for j, line := range captions[i] {
			drawProofLabel(img, proofPadding, int(top)+16+j*legendLineHeight, line, color.Black)
		}
		ty := top + headerH + proofPadding - bounds.minY
		drawProofLayout(img, l, opts, tx, ty)
		geometry.Rows = append(geometry.Rows, screenRow(l, tx, ty))
		legendY := top + headerH + rowPlotH
		for j, line := range legends[i] {
			drawProofLabel(img, proofPadding, int(legendY)+(j+1)*legendLineHeight-4, line, color.Black)
		}
		top += headerH + rowPlotH + float64(len(legends[i])*legendLineHeight)
		if i < len(layouts)-1 {
			drawProofLine(img, proofPadding, int(top)-1, int(width)-proofPadding, int(top)-1, color.RGBA{210, 210, 210, 255})
		}
	}
	return img, geometry, nil
}
