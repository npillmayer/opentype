package ot

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"iter"
)

// Reading bytes from a font's binary representation

var errBufferBounds = errors.New("internal inconsistency: buffer bounds error")

func u16(b []byte) uint16 {
	_ = b[1] // Bounds check hint to compiler
	return uint16(b[0])<<8 | uint16(b[1])<<0
}

func u32(b []byte) uint32 {
	_ = b[3] // Bounds check hint to compiler
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])<<0
}

// ---Locations, i.e. byte segments/slices -----------------------------------

// NavLocation is a position at a byte within a font's binary data.
// It represents the start of a segment/slice of binary data.
//
// NavLocation is always the final link of a chain of Navigator calls, giving access to
// underlying (unstructured) font data. It is the client's responsibility to interpret the
// structure and impose it onto the NavLocation's bytes.
//
// If an error occured somewhere along a chain of navigation calls, the finally resulting NavLocation
// may be of size 0.
type NavLocation interface {
	Size() int                  // size in bytes
	Bytes() []byte              // return as a byte slice
	Slice(int, int) NavLocation // return a sub-segment of this location
	U16(int) uint16             // convenience access to 16 bit data at byte index
	U32(int) uint32             // convenience access to 32 bit data at byte index
	Glyphs() []GlyphIndex       // convenience conversion to slice of glyphs
}

// binarySegm is a segment of byte data.
// It implements the Location interface. We use it throughout this module to
// naviagte the font's binary data.
type binarySegm []byte

func (b binarySegm) Size() int {
	return len(b)
}

func (b binarySegm) Bytes() []byte {
	return b
}

// return a sub-segment of this location
func (b binarySegm) Slice(from int, to int) NavLocation {
	if from < 0 {
		from = 0
	}
	if to > len(b) {
		to = len(b)
	}
	return b[from:to]
}

func (b binarySegm) Reader() io.Reader {
	return bytes.NewReader(b)
}

func (b binarySegm) U16(i int) uint16 {
	n, err := b.u16(i)
	if err != nil {
		return 0
	}
	return n
}

func (b binarySegm) U32(i int) uint32 {
	n, err := b.u32(i)
	if err != nil {
		return 0
	}
	return n
}

// convenience conversion to slice of glyphs
func (b binarySegm) Glyphs() []GlyphIndex {
	l := len(b)
	if l|0x1 > 0 {
		l += 1
	}
	glyphs := make([]GlyphIndex, l/2)
	j := 0
	for i := 0; i < len(b); i += 2 {
		glyphs[j] = GlyphIndex(b[i])<<8 + GlyphIndex(b[i+1])
		j++
	}
	return glyphs

}

func asU16Slice(b binarySegm) []uint16 {
	r := make([]uint16, len(b)/2+1)
	j := 0
	for i := 0; i < len(b); i += 2 {
		r[j] = uint16(b[i])<<8 + uint16(b[i+1])
		j++
	}
	return r
}

// return an unsigned integer as an array of two bytes.
func uintBytes(n uint16) binarySegm {
	return binarySegm{byte(n >> 8 & 0xff), byte(n & 0xff)}
}

// view returns n bytes at the given offset.
// The byte segment returned is a sub-slice of b.
func (b binarySegm) view(offset, n int) (binarySegm, error) {
	if offset < 0 || n <= 0 || offset+n > len(b) {
		return nil, errBufferBounds
	}
	return b[offset : offset+n], nil
}

// u16 returns the uint16 in b at the relative offset i.
func (b binarySegm) u16(i int) (uint16, error) {
	buf, err := b.view(i, 2)
	if err != nil {
		return 0, err
	}
	return u16(buf), nil
}

// u32 returns the uint32 in b at the relative offset i.
func (b binarySegm) u32(i int) (uint32, error) {
	buf, err := b.view(i, 4)
	if err != nil {
		return 0, err
	}
	return u32(buf), nil
}

// --- Ranges of glyphs ------------------------------------------------------

// GlyphRange is a type frequently used by sub-tables of layout tables (GPOS and GSUB).
// If an input glyph g is contained in the range, and index and true is returned,
// false otherwise.
type GlyphRange interface {
	Match(g GlyphIndex) (int, bool) // is glyph ID g in range?
	ByteSize() int
}

type glyphRangeArray struct {
	is32     bool // keys are 32 bit
	count    int  // number of glyph keys
	data     binarySegm
	byteSize int
}

// glyphRangeArrays have entries stored as a block of consecutive keys.
// glyphRangeArrays return the index of the key in the range table.
// 0 is a valid return value.
func (r *glyphRangeArray) Match(g GlyphIndex) (int, bool) {
	if r.count <= 0 {
		return 0, false
	}
	if r.is32 {
		for i := 0; i < r.count; i++ {
			k, err := r.data.u32(i * 4)
			if err != nil {
				return 0, false
			} else if GlyphIndex(k) == g {
				return i, true
			}
		}
	} else {
		for i := 0; i < r.count; i++ {
			k, err := r.data.u16(i * 2)
			if err != nil {
				return 0, false
			} else if GlyphIndex(k) == g {
				return i, true
			}
		}
	}
	return 0, false
}

type rangeRecord struct {
	from, to GlyphIndex
	index    uint16
}

func (r *glyphRangeArray) ByteSize() int {
	return r.byteSize
}

type glyphRangeRecords struct {
	is32     bool // keys are 32 bit
	count    int  // number of range records
	data     binarySegm
	byteSize int
}

// glyphRangeRecords have entries stored as range records.
// glyphRangeRecords return the index of the key in the range table.
// 0 is a valid return value.
func (r *glyphRangeRecords) Match(g GlyphIndex) (int, bool) {
	tracer().Debugf("glyph range lookup of glyph ID %d", g)
	if r.count <= 0 {
		return 0, false
	}
	record := rangeRecord{}
	if r.is32 {
		for i := 0; i < r.count; i++ {
			k, err := r.data.u32(i * (4 + 4 + 2))
			if err != nil {
				return 0, false
			}
			record.from = GlyphIndex(k)
			k, _ = r.data.u32(i*(2+2+2) + 4)
			record.to = GlyphIndex(k)
			v, _ := r.data.u16(i*(2+2+2) + 6)
			record.index = v
			if record.from <= g && g <= record.to {
				return int(record.index + uint16(g-record.from)), true
			}
		}
	} else {
		tracer().Debugf("range of %d records", r.count)
		for i := 0; i < r.count; i++ {
			k, err := r.data.u16(i * (2 + 2 + 2))
			if err != nil {
				return 0, false
			}
			record.from = GlyphIndex(k)
			k, _ = r.data.u16(i*(2+2+2) + 2)
			record.to = GlyphIndex(k)
			k, _ = r.data.u16(i*(2+2+2) + 4)
			record.index = k
			tracer().Debugf("from %d to %d => %d...", record.from, record.to, record.index)
			if record.from <= g && g <= record.to {
				return int(record.index + uint16(g-record.from)), true
			}
		}
	}
	return 0, false
}

func (r *glyphRangeRecords) ByteSize() int {
	return r.byteSize
}

// --- Tag list --------------------------------------------------------------

type tagList struct {
	Count int
	link  NavLink
}

func parseTagList(b binarySegm) tagList {
	if len(b) < 2 {
		return tagList{Count: 0}
	}

	count := int(u16(b))

	// Validate against reasonable limit
	if count > MaxTagListCount {
		tracer().Errorf("tag list count %d exceeds maximum %d", count, MaxTagListCount)
		return tagList{Count: 0}
	}

	// Validate against buffer size (each tag is 4 bytes)
	requiredSize := 2 + count*4
	if requiredSize > len(b) {
		tracer().Errorf("tag list: count %d requires %d bytes, have %d",
			count, requiredSize, len(b))
		return tagList{Count: 0}
	}

	tl := tagList{Count: count}
	tl.link = link16{
		base:   b,
		offset: 2,
	}
	return tl
}

func (l tagList) Tag(i int) Tag {
	const taglen = 4
	if b := l.link.Jump(); len(b.Bytes()) >= (i+1)*taglen {
		if n, err := binarySegm(b.Bytes()).u32(i * taglen); err == nil {
			return Tag(n)
		}
	}
	return Tag(0)
}

// --- Link ------------------------------------------------------------------

// NavLink is a type to represent the transfer between one Navigator item and
// another. Clients may use it to either arrive at the binary segment of the
// destination (call Jump) or to receive the destination as a Navigator item
// (call Navigate).
//
// Name returns the class name of the link's destination. IsNull is used to check
// if this NavLink represents a link to a valid destination.
type NavLink interface {
	Base() NavLocation   // source location
	Jump() NavLocation   // destination location
	IsNull() bool        // is this a valid link?
	Navigate() Navigator // interpret destination as an OpenType structure element
	Name() string        // OpenType structure name of destination
}

// parseLink16 parses a byte array which is presumed to be a map16 entry, where a `uint16`
// value is interpreted as a navigation link to another entity.
// `offset` is the number of bytes from the beginning of the base segment to the
// beginning of the value segment. It usually is the size of the 'key' in bytes, but
// no semantics is enforced.
func parseLink16(b binarySegm, offset int, base binarySegm, target string) (NavLink, error) {
	if len(b) < offset+2 { // room for key + value ?
		return link16{}, errBufferBounds
	}
	n, _ := b.u16(offset) // retrieve the value

	// Validate offset points to be within base bounds (offset 0 is valid NULL pointer)
	if n > 0 && int(n) > len(base) {
		return link16{}, fmt.Errorf("offset16 to %s out of bounds: %d > %d", target, n, len(base))
	}

	return link16{
		target: target,
		base:   base,
		offset: n,
	}, nil
}

// func makeLink16(b fontBinSegm, offset uint16, base fontBinSegm, target string) Link {
func makeLink16(offset uint16, base binarySegm, target string) NavLink {
	return link16{
		target: target,
		base:   base,
		offset: offset,
	}
}

type link16 struct {
	err    error
	target string
	base   binarySegm
	offset uint16
}

func (l16 link16) IsNull() bool {
	if l16.err != nil {
		return true
	}
	return len(l16.base) == 0
}

func (l16 link16) Name() string {
	return l16.target
}

func (l16 link16) Base() NavLocation {
	return l16.base
}

func (l16 link16) Jump() NavLocation {
	tracer().Debugf("jump to %s", l16.target)
	if l16.err != nil {
		return binarySegm{}
	}
	if l16.offset > uint16(len(l16.base)) {
		tracer().Debugf("base has size %d", len(l16.base))
		tracer().Debugf("link to %d", l16.offset)
		tracer().Debugf("offset16 location out of table bounds")
		return binarySegm{}
	}
	return l16.base[l16.offset:]
}

func (l16 link16) Navigate() Navigator {
	if l16.err != nil {
		return null(l16.err)
	}
	return NavigatorFactory(l16.target, l16.Jump(), l16.base)
}

func parseLink32(b binarySegm, offset int, base binarySegm, target string) (NavLink, error) {
	if len(b) < offset+4 {
		return link32{}, errBufferBounds
	}
	n, _ := b.u32(offset)
	// Validate offset points within base bounds (offset 0 is valid NULL pointer)
	if n > 0 && int(n) > len(base) {
		return link32{}, fmt.Errorf("offset32 to %s out of bounds: %d > %d", target, n, len(base))
	}
	return link32{
		target: target,
		base:   base,
		offset: n,
	}, nil
}

type link32 struct {
	err    error
	target string
	base   binarySegm
	offset uint32
}

func (l32 link32) IsNull() bool {
	if l32.err != nil {
		return true
	}
	return len(l32.base) == 0
}

func (l32 link32) Name() string {
	return l32.target
}

func (l32 link32) Base() NavLocation {
	return l32.base
}

func makeLink32(offset uint32, base binarySegm, target string) NavLink {
	return link32{
		target: target,
		base:   base,
		offset: offset,
	}
}

func (l32 link32) Jump() NavLocation {
	tracer().Debugf("jump to %s", l32.target)
	if l32.err != nil {
		return binarySegm{}
	}
	if l32.offset > uint32(len(l32.base)) {
		tracer().Debugf("base has size %d", len(l32.base))
		tracer().Debugf("link to %d", l32.offset)
		tracer().Debugf("offset32 location out of table bounds")
		return binarySegm{}
	}
	return l32.base[l32.offset:]
}

func (l32 link32) Navigate() Navigator {
	if l32.err != nil {
		return null(l32.err)
	}
	return NavigatorFactory(l32.target, l32.Jump(), l32.base)
}

// --- Arrays ----------------------------------------------------------------

// array is a type for a linear sequence of equal-sized records.
type array struct {
	name       string
	target     string
	recordSize int
	length     int
	loc        binarySegm
}

func ParseList(b binarySegm, N int, recordSize int) NavList {
	return array{
		recordSize: recordSize,
		length:     N,
		loc:        b,
	}
}

func viewArray16(b binarySegm) array {
	if b.Size()&0x1 != 0 {
		tracer().Errorf("cannot create array16: size not aligned")
		return array{}
	}
	n := b.Size() / 2
	return array{
		recordSize: 2,
		length:     n,
		loc:        b,
	}
}

func parseArray(b binarySegm, offset int, recordSize int, name, target string) (array, error) {
	if len(b) < offset+2 {
		return array{name: name, target: target}, errBufferBounds
	}
	n, err := b.u16(offset)
	if err != nil {
		return array{}, err
	}

	// Validate count against buffer size
	headerSize := offset + 2
	requiredSize := headerSize + int(n)*recordSize
	if requiredSize > len(b) {
		return array{}, fmt.Errorf("array %s: count %d * recordSize %d requires %d bytes, have %d",
			name, n, recordSize, requiredSize, len(b))
	}

	return array{
		name:       name,
		target:     target,
		recordSize: recordSize,
		length:     int(n),
		loc:        b[offset+2:],
	}, nil
}

func parseArray16(b binarySegm, offset int, name, target string) (array, error) {
	if len(b) < offset+2 {
		return array{name: name, target: target}, errBufferBounds
	}
	n, err := b.u16(offset)
	if err != nil {
		return array{}, err
	}

	// Validate count against buffer size (record size is 2 for offsets)
	headerSize := offset + 2
	requiredSize := headerSize + int(n)*2
	if requiredSize > len(b) {
		return array{}, fmt.Errorf("array16 %s: count %d requires %d bytes, have %d",
			name, n, requiredSize, len(b))
	}

	return array{
		name:       name,
		target:     target,
		recordSize: 2,
		length:     int(n),
		loc:        b[offset+2:],
	}, nil
}

func viewArray(b binarySegm, recordSize int) array {
	N := b.Size() / recordSize
	tracer().Debugf("view array[%d](%d)", N, recordSize)
	return array{
		recordSize: recordSize,
		length:     N,
		loc:        b,
	}
}

func (a array) Name() string {
	return a.name
}

// Size of array a in bytes.
func (a array) Size() int {
	return a.length * a.recordSize
}

// Len returns the number of entries in the list.
func (a array) Len() int {
	return a.length
}

// Get returns item #i as a byte location.
func (a array) Get(i int) NavLocation {
	if i < 0 || (i+1)*a.recordSize > len(a.loc.Bytes()) {
		i = 0
	}
	b, _ := a.loc.view(i*a.recordSize, a.recordSize)
	return b
}

func (a array) All() []NavLocation {
	r := make([]NavLocation, a.length)
	for i := 0; i < a.length; i++ {
		x := a.Get(i)
		r = append(r, x)
	}
	return r
}

// VarArray is a type for arrays of variable length records, which in turn may point to nested
// arrays of (variable size) records.
type VarArray interface {
	Get(i int, deep bool) (NavLocation, error) // get record at index i; if deep: query nested arrays
	Size() int                                 // get the number of entries
}

type varArray struct {
	name         string
	ptrs         array
	indirections int
	base         binarySegm
}

// ParseVarArray interprets a byte sequence as a `VarArray`.
func ParseVarArray(loc NavLocation, sizeOffset, arrayDataGap int, name string) VarArray {
	return parseVarArray16(loc.Bytes(), sizeOffset, arrayDataGap, 1, name)
}

func parseVarArray16(b binarySegm, szOffset, gap, indirections int, name string) varArray {
	minSize := szOffset + gap + 2
	if len(b) < minSize {
		tracer().Errorf("byte segment too small to parse variable array")
		return varArray{}
	}
	//
	// Enforce maximum indirection depth to prevent stack overflow
	if indirections > MaxIndirectionDepth {
		tracer().Errorf("varArray %s: indirection depth %d exceeds maximum %d",
			name, indirections, MaxIndirectionDepth)
		return varArray{}
	}

	cnt, _ := b.u16(szOffset)

	// Validate count against buffer size (each pointer is 2 bytes)
	requiredSize := szOffset + gap + int(cnt)*2
	if requiredSize > len(b) {
		tracer().Errorf("varArray %s: count %d requires %d bytes, have %d",
			name, cnt, requiredSize, len(b))
		return varArray{}
	}

	va := varArray{name: name, indirections: indirections, base: b}
	va.ptrs = array{recordSize: 2, length: int(cnt), loc: b[szOffset+gap:]}
	tracer().Debugf("parsing VarArray of size %d with %d indirections", cnt, indirections)
	return va
}

// Get looks up index i within the cascading arrays of va. If deep is false, only
// the top-level array will be queried.
func (va varArray) Get(i int, deep bool) (b NavLocation, err error) {
	var a array = va.ptrs
	var indirect = va.indirections
	if !deep {
		indirect = 1
	}
	base := va.base
	for j := 0; j < indirect; j++ {
		b = a.Get(i) // TODO will this create an infinite loop in case of error?
		tracer().Debugf("varArray->Get(%d|%d), a = %v", i, a.length, binarySegm(a.loc.Bytes()[:20]).Glyphs())
		tracer().Debugf("b = %d, %d to go", b.U16(0), va.indirections-1-j)
		if b.U16(0) == 0 {
			tracer().Debugf("link to ptrs-data is NULL, empty array")
			return binarySegm{}, nil
		}
		if j < va.indirections {
			link := makeLink16(b.U16(0), base, "Sequence")
			b = link.Jump()
			if j+1 < va.indirections {
				a, err = parseArray16(b.Bytes(), 0, "var-array", "var-array-entry")
				tracer().Debugf("new a has size %d, is %v", a.length, binarySegm(a.loc.Bytes()[:20]).Glyphs())
			}
		}
	}
	tracer().Debugf("varArray result = %v", asU16Slice(binarySegm(b.Bytes()[:min(20, 2*b.Size())])))
	return b, err
}

func (va varArray) Size() int {
	return va.ptrs.length
}

var _ VarArray = varArray{}

// --- Tag record map --------------------------------------------------------

// tagRecordMap16 is a type for sub-tables which map from a tag to a target.
// `record` points to a struct essentially holding a slice of bytes, which is interpreted
// as a fixed size array of entries.
type tagRecordMap16 struct {
	name    string
	target  string
	base    binarySegm
	records array
}

// makeTagRecordMap16 creates a map-like interpretation on a slice of bytes.
//
// | Type      | Name         | Descr.                      |
// |-----------|--------------|-----------------------------|
// | offset    | Some Info    | Additional opaque data      |
// | uint16    | Count        | # Records                   |
// | x-Records | Array[Count] | Array of records or indices |
//
// For tag record maps, the entries (x-Records) are segments of bytes, which in turn
// are interpreted as a key + a value. The key is expected to be a 4-byte tag.
// `offset`may be 0.
func makeTagRecordMap16(name, target string, b, base binarySegm, offset, N int) tagRecordMap16 {
	m := tagRecordMap16{
		name:   name,
		target: target,
		base:   base,
	}
	const recordSize = 6 // Tag=4 bytes + offset-value=2 bytes (= map16)
	const countSize = 2  // count is uint16
	arraySize := N * recordSize
	eob := offset + countSize + arraySize
	if b == nil {
		b = make(binarySegm, eob)
		// need to set the count value to N
		var low byte = byte(uint16(N) & 0xff)
		var high byte = byte(uint16(N>>8) & 0xff)
		b.Bytes()[0] = low
		b.Bytes()[1] = high
	} else if eob > len(b) {
		tracer().Errorf("byte buffer too small for tag record map")
		return tagRecordMap16{}
	}
	n, _ := b.u16(offset)
	if int(n) != N {
		tracer().Errorf("invalid count %d for tag record map", n)
		panic("record count n not equal to given count N")
	}
	arrBase := b[offset+countSize : eob]
	m.records = viewArray(arrBase, recordSize)
	return m
}

// recsize is the byte size of the record entry not including the Tag.
func parseTagRecordMap16(b binarySegm, offset int, base binarySegm, name, target string) tagRecordMap16 {
	if len(b) < offset+2 {
		tracer().Errorf("buffer too small for tag record map")
		return tagRecordMap16{}
	}
	N, err := b.u16(offset)
	if err != nil {
		return tagRecordMap16{}
	}
	// Apply reasonable limit based on context
	var maxCount int
	switch name {
	case "ScriptList":
		maxCount = MaxScriptCount
	case "FeatureList":
		maxCount = MaxFeatureCount
	default:
		maxCount = MaxRecordMapCount
	}
	if int(N) > maxCount {
		tracer().Errorf("tag record map %s: count %d exceeds maximum %d", name, N, maxCount)
		return tagRecordMap16{}
	}
	const recordSize = 6 // validate count against buffer size (Tag=4 bytes + offset=2 bytes = 6)
	const countSize = 2  // count is uint16
	requiredSize := offset + countSize + int(N)*recordSize
	if requiredSize > len(b) {
		tracer().Errorf("tag record map %s: count %d requires %d bytes, have %d",
			name, N, requiredSize, len(b))
		return tagRecordMap16{}
	}
	tracer().Debugf("view on tag record map with %d entries", N)
	return makeTagRecordMap16(name, target, b, base, offset, int(N))
	// TODO remove this when tests pass
	// m := tagRecordMap16{
	// 	name:   name,
	// 	target: target,
	// 	base:   base,
	// }
	// arrBase := b[offset+2 : offset+2+int(N)*recordSize]
	// m.records = viewArray(arrBase, recordSize)
	// return m
}

// Lookup returns the bytes referenced by m[Tag(n)]
func (m tagRecordMap16) Lookup(n uint32) NavLocation {
	tag := Tag(n)
	return m.LookupTag(tag).Jump()
}

// Lookup returns the link associated with a given tag.
//
// TODO binary search with |N| > ?
func (m tagRecordMap16) LookupTag(tag Tag) NavLink {
	if len(m.base) == 0 {
		tracer().Debugf("tag record map has null-base")
		return link16{}
	}
	tracer().Debugf("tag record map has %d entries", m.records.length)
	for i := 0; i < m.records.length; i++ {
		b := m.records.Get(i)
		rtag := MakeTag(b.Bytes()[:4])
		tracer().Debugf("testing for tag = %s", rtag)
		if tag == rtag {
			tracer().Debugf("tag record lookup found tag (%s)", rtag)
			link, err := parseLink16(b.Bytes(), 4, m.base, m.target)
			if err != nil {
				return link16{}
			}
			tracer().Debugf("    record links %s from %d", m.target, link.Base().U16(0))
			return link
		}
	}
	return link16{}
}

// Tags returns all the tags which the map uses as keys.
func (m tagRecordMap16) Tags() []Tag {
	tracer().Debugf("tag record map has %d entries", m.records.length)
	tags := make([]Tag, 0, 3)
	for i := 0; i < m.records.length; i++ {
		b := m.records.Get(i)
		tag := MakeTag(b.Bytes()[:4])
		tracer().Debugf("  Tag = (%s)", tag)
		tags = append(tags, tag)
	}
	return tags
}

func (m tagRecordMap16) Name() string {
	return m.name
}

func (m tagRecordMap16) Len() int {
	return m.records.length
}

func (m tagRecordMap16) Get(i int) (Tag, NavLink) {
	b := m.records.Get(i)
	const sizeOfMapKey = 4 // tags have size of 4 bytes
	tag := MakeTag(b.Bytes()[:sizeOfMapKey])
	link, err := parseLink16(b.Bytes(), sizeOfMapKey, m.base, m.target)
	if err != nil {
		return 0, link16{}
	}
	return tag, link
}

func (m tagRecordMap16) IsTagRecordMap() bool {
	return true
}

func (m tagRecordMap16) AsTagRecordMap() TagRecordMap {
	return m
}

// Interface TagRecordMap: `Subset` subsets a tag record map with the indices given by a NavList.
//
// Example map:
// | Tag      | Target   |
// |----------|----------|
// | tag 1    | link 1   |
// | tag 2    | link 2   |
// | tag 3    | link 3   |
// | tag 4    | link 4   |
//
// Example list: [1,  3]
//
// Subset:
// | Tag      | Target   |
// |----------|----------|
// | tag 1    | link 1   |
// | tag 3    | link 3   |
//
// Usually the source map is a projection onto the font's bytes. The subset map has to
// allocate a new byte array for the subset.
func (m tagRecordMap16) Subset(indices NavList) TagRecordMap {
	if indices == nil || indices.Len() == 0 {
		return tagRecordMap16{}
	}
	N := indices.Len() // will allocate space for N records
	subset := makeTagRecordMap16(m.name, m.target, m.base, nil, 0, N)
	records := subset.records.loc[:0] // not sure we will use all slots
	for i := range N {
		index := indices.Get(i).U16(0)
		if int(index) >= m.records.length {
			//continue
			panic("subset of tag record map: cannot apply link > |record array|")
		}
		bytes := m.records.Get(int(index))
		records = append(records, bytes.Bytes()...)
	}
	return subset
}

func (m tagRecordMap16) Range() iter.Seq2[Tag, NavLink] {
	return func(yield func(Tag, NavLink) bool) {
		for i := range m.Len() {
			tag, link := m.Get(i)
			if !yield(tag, link) {
				return
			}
		}
	}
}

// ----------------------------------------------------------------------

// This will make a map of Tag to link16 conform to TagRecordMap
type mapWrapper struct {
	//names nameNames // TODO de-couple from  table 'name' -> done
	m    map[Tag]link16
	name string
}

func (mw mapWrapper) Name() string {
	return mw.name
}

func (mw mapWrapper) Len() int {
	return len(mw.m)
}

func (mw mapWrapper) LookupTag(tag Tag) NavLink {
	if link, ok := mw.m[tag]; ok {
		tracer().Debugf("NameRecord link for %x = %v", tag, link)
		return link
	}
	return nullLink(fmt.Sprintf("no name for key %d", tag))
}

// Lookup returns the bytes referenced by m[Tag(n)]
func (mw mapWrapper) Lookup(n uint32) NavLocation {
	tag := Tag(n)
	return mw.LookupTag(tag).Jump()
}

func (mw mapWrapper) Tags() []Tag {
	l := len(mw.m) // we will allocate a slice of length l for tags
	tags := make([]Tag, 0, l)
	for k := range mw.m {
		tags = append(tags, k)
	}
	return tags
}

// Get does nothing
func (mw mapWrapper) Get(int) (Tag, NavLink) {
	return 0, link16{}
}

func (mw mapWrapper) IsTagRecordMap() bool {
	return true
}

func (mw mapWrapper) AsTagRecordMap() TagRecordMap {
	return mw
}

// Interface TagRecordMap
func (mw mapWrapper) Subset(indices NavList) TagRecordMap {
	panic("not implemented")
}

// Interface TagRecordMap
// Range() iter.Seq2[Tag, NavLink] // range over sequence of tag-record pairs
func (mw mapWrapper) Range() iter.Seq2[Tag, NavLink] {
	return func(yield func(Tag, NavLink) bool) {
		for i := range mw.Len() {
			tag, link := mw.Get(i)
			if !yield(tag, link) {
				return
			}
		}
	}
}

// u16List implements the NavList interface. It represents a list/array of
// u16 values.
type u16List []uint16

func (u16l u16List) Name() string {
	return "<unknown>"
}

func (u16l u16List) Len() int {
	return len(u16l)
}

func (u16l u16List) Get(i int) NavLocation {
	if i < 0 || i >= len(u16l) {
		return binarySegm{}
	}
	return binarySegm{byte(u16l[i] >> 8 & 0xff), byte(u16l[i] & 0xff)}
}

func (u16l u16List) All() []NavLocation {
	r := make([]NavLocation, len(u16l))
	for i, x := range u16l {
		r[i] = binarySegm([]byte{byte(x >> 8 & 0xff), byte(x & 0xff)})
	}
	return r
}

// ---------------------------------------------------------------------------

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
