package ot

import "fmt"

type baseTagListView struct {
	raw   binarySegm
	count int
}

func parseBaseTagListView(raw binarySegm, offset int) (baseTagListView, error) {
	if offset < 0 || offset+2 > len(raw) {
		return baseTagListView{}, fmt.Errorf("BASE BaseTagList offset out of bounds")
	}
	count := int(raw.U16(offset))
	size := count * 4
	if size < 0 || offset+2+size > len(raw) {
		return baseTagListView{}, fmt.Errorf("BASE BaseTagList records out of bounds")
	}
	return baseTagListView{raw: raw[offset+2 : offset+2+size], count: count}, nil
}

func (v baseTagListView) All() []Tag {
	if v.count == 0 {
		return nil
	}
	tags := make([]Tag, v.count)
	for i := 0; i < v.count; i++ {
		start := i * 4
		tags[i] = Tag(u32(v.raw[start : start+4]))
	}
	return tags
}

type baseTagOffset16View struct {
	raw   binarySegm
	count int
}

func parseTagOffset16View(raw binarySegm, countOffset int) (baseTagOffset16View, error) {
	if countOffset < 0 || countOffset+2 > len(raw) {
		return baseTagOffset16View{}, fmt.Errorf("BASE tag-offset view count out of bounds")
	}
	count := int(raw.U16(countOffset))
	size := count * 6
	start := countOffset + 2
	if size < 0 || start+size > len(raw) {
		return baseTagOffset16View{}, fmt.Errorf("BASE tag-offset records out of bounds")
	}
	return baseTagOffset16View{raw: raw[start : start+size], count: count}, nil
}

func (v baseTagOffset16View) Record(i int) (Tag, uint16, bool) {
	if i < 0 || i >= v.count {
		return 0, 0, false
	}
	start := i * 6
	tag := Tag(u32(v.raw[start : start+4]))
	off := u16(v.raw[start+4 : start+6])
	return tag, off, true
}

func (v baseTagOffset16View) LookupOffset(tag Tag) (uint16, bool) {
	lo, hi := 0, v.count-1
	for lo <= hi {
		mid := (lo + hi) / 2
		rtag, off, _ := v.Record(mid)
		switch {
		case rtag == tag:
			return off, true
		case rtag < tag:
			lo = mid + 1
		default:
			hi = mid - 1
		}
	}
	return 0, false
}

type offset16View struct {
	raw   binarySegm
	count int
}

func parseOffset16View(raw binarySegm, countOffset int) (offset16View, error) {
	if countOffset < 0 || countOffset+2 > len(raw) {
		return offset16View{}, fmt.Errorf("BASE offset view count out of bounds")
	}
	count := int(raw.U16(countOffset))
	size := count * 2
	start := countOffset + 2
	if size < 0 || start+size > len(raw) {
		return offset16View{}, fmt.Errorf("BASE offset records out of bounds")
	}
	return offset16View{raw: raw[start : start+size], count: count}, nil
}

func (v offset16View) At(i int) (uint16, bool) {
	if i < 0 || i >= v.count {
		return 0, false
	}
	start := i * 2
	return u16(v.raw[start : start+2]), true
}

func viewBaseAxis(base binarySegm, offset uint16) (*BaseAxis, bool) {
	if offset == 0 || int(offset) >= len(base) {
		return nil, false
	}
	raw := base[offset:]
	if len(raw) < 4 {
		return &BaseAxis{raw: raw, err: fmt.Errorf("BASE axis table too small")}, false
	}
	tagListOffset := int(raw.U16(0))
	scriptListOffset := int(raw.U16(2))
	axis := &BaseAxis{raw: raw}
	var err error
	if tagListOffset > 0 {
		axis.baselineTags, err = parseBaseTagListView(raw, tagListOffset)
		if err != nil {
			axis.err = err
		}
	}
	axis.scripts, err = parseTagOffset16View(raw, scriptListOffset)
	if err != nil {
		if axis.err == nil {
			axis.err = err
		}
		return axis, false
	}
	return axis, true
}

func viewBaseScript(axisRaw binarySegm, offset uint16) (*BaseScript, bool) {
	if offset == 0 || int(offset) >= len(axisRaw) {
		return nil, false
	}
	raw := axisRaw[offset:]
	if len(raw) < 6 {
		return &BaseScript{raw: raw, err: fmt.Errorf("BASE BaseScript table too small")}, false
	}
	langs, err := parseTagOffset16View(raw, 4)
	s := &BaseScript{
		raw:              raw,
		baseValuesOffset: raw.U16(0),
		defaultMinMax:    raw.U16(2),
		langSysMinMax:    langs,
		err:              err,
	}
	return s, err == nil
}

func viewBaseValues(raw binarySegm) (*BaseValues, bool) {
	if len(raw) < 4 {
		return &BaseValues{raw: raw, err: fmt.Errorf("BASE BaseValues table too small")}, false
	}
	coords, err := parseOffset16View(raw, 2)
	v := &BaseValues{
		raw:                  raw,
		defaultBaselineIndex: raw.U16(0),
		coords:               coords,
		err:                  err,
	}
	return v, err == nil
}

func viewMinMax(raw binarySegm) (*MinMax, bool) {
	if len(raw) < 6 {
		return &MinMax{raw: raw, err: fmt.Errorf("BASE MinMax table too small")}, false
	}
	features, err := parseTagOffset16View(raw, 4)
	m := &MinMax{
		raw:             raw,
		minCoordOffset:  raw.U16(0),
		maxCoordOffset:  raw.U16(2),
		featureMinMaxes: features,
		err:             err,
	}
	return m, err == nil
}

func viewFeatureMinMax(raw binarySegm) (*FeatureMinMax, bool) {
	if len(raw) < 4 {
		return &FeatureMinMax{raw: raw, err: fmt.Errorf("BASE FeatureMinMax table too small")}, false
	}
	f := &FeatureMinMax{
		raw:            raw,
		minCoordOffset: raw.U16(0),
		maxCoordOffset: raw.U16(2),
	}
	return f, true
}

func viewBaseCoord(raw binarySegm) (*BaseCoord, bool) {
	if len(raw) < 4 {
		return &BaseCoord{raw: raw, err: fmt.Errorf("BASE BaseCoord table too small")}, false
	}
	format := raw.U16(0)
	c := &BaseCoord{raw: raw, format: format}
	switch format {
	case 1:
		return c, true
	case 2:
		if len(raw) < 8 {
			c.err = fmt.Errorf("BASE BaseCoord format-2 table too small")
			return c, false
		}
		return c, true
	case 3:
		if len(raw) < 6 {
			c.err = fmt.Errorf("BASE BaseCoord format-3 table too small")
			return c, false
		}
		return c, true
	default:
		c.err = fmt.Errorf("unsupported BASE BaseCoord format %d", format)
		return c, false
	}
}
