package otquery

import (
	"fmt"
	"iter"

	"github.com/npillmayer/opentype/ot"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/text/encoding/unicode"
)

const (
	nameHeaderSize = 6
	nameRecordSize = 12
)

// nameKey identifies a NameRecord entry in OpenType table 'name'.
// The key follows the OpenType NameRecord fields directly.
type nameKey struct {
	Platform PlatformID
	Encoding EncodingID
	Language uint16      // not supported
	Name     sfnt.NameID // see https://pkg.go.dev/golang.org/x/image/font/sfnt#NameID
}

// type nameEntry struct {
// 	key   nameKey
// 	value string
// }

type PlatformID uint16

const (
	PlatformIDUnicode   PlatformID = 0
	PlatformIDMacintosh PlatformID = 1 // not supported
	PlatformIDWindows   PlatformID = 3
)

type EncodingID uint16

const (
	EncodingIDUnicodeBMP    EncodingID = 3
	EncodingIDWindowsSymbol EncodingID = 0 // for now we will not support symbol fonts
	EncodingIDWindowsBMP    EncodingID = 1
)

// NamesRange yields decoded `(nameID, value)` pairs from a font's OpenType
// `name` table.
//
// Only currently supported encodings are yielded (Unicode BMP and Windows BMP),
// and malformed or out-of-bounds records are skipped.
func NamesRange(otf *ot.Font) iter.Seq2[sfnt.NameID, string] {
	names := checkNameTableSafe(otf)
	return func(yield func(sfnt.NameID, string) bool) {
		if names == nil {
			return
		}
		binary := names.Binary()
		count := int(u16(binary[2:4])) // number of name records
		stringStorageOffset := int(u16(binary[4:6]))
		for i := range count {
			recordSlice := binary[nameHeaderSize+i*nameRecordSize : nameHeaderSize+(i+1)*nameRecordSize]
			key := nameKey{
				Platform: PlatformID(u16(recordSlice[0:2])),
				Encoding: EncodingID(u16(recordSlice[2:4])),
				Language: u16(recordSlice[4:6]),
				Name:     sfnt.NameID(u16(recordSlice[6:8])),
			}
			if !isSupportedNameEncoding(key) {
				continue
			}
			strLen := int(u16(recordSlice[8:10]))
			recordOffset := int(u16(recordSlice[10:12]))
			start := stringStorageOffset + recordOffset
			end := start + strLen
			if start < 0 || strLen < 0 || end > len(binary) {
				continue
			}
			stringValue, err := decodeNameUTF16(binary[start:end])
			if err != nil || stringValue == "" {
				continue
			}
			if !yield(key.Name, stringValue) {
				return
			}
		}
	}
}

// checkNameTableSafe checks if the name table is safe to use, i.e. no out-of-bounds access,
// no empty tables, etc.
func checkNameTableSafe(otf *ot.Font) ot.Table {
	if otf == nil {
		return nil
	}
	table := otf.Table(ot.T("name"))
	if table == nil {
		tracer().Debugf("no name table found in font")
		return nil
	}
	b := table.Binary()
	if len(b) < nameHeaderSize {
		tracer().Debugf("name table too short: %d", len(b))
		return nil
	}
	count := int(u16(b[2:4]))
	strOff := int(u16(b[4:6]))
	if strOff < 0 || strOff > len(b) {
		tracer().Debugf("name table invalid string offset: %d", strOff)
		return nil
	}
	recordsEnd := nameHeaderSize + count*nameRecordSize
	if recordsEnd > len(b) {
		tracer().Debugf("name table record section out of bounds: count=%d", count)
		return nil
	}
	return table
}

func isSupportedNameEncoding(key nameKey) bool {
	// Keep current behavior: decode Unicode BMP + Windows BMP entries only.
	return (key.Platform == PlatformIDUnicode && key.Encoding == EncodingIDUnicodeBMP) ||
		(key.Platform == PlatformIDWindows && key.Encoding == EncodingIDWindowsBMP)
}

func decodeNameUTF16(str []byte) (string, error) {
	enc := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
	decoder := enc.NewDecoder()
	s, err := decoder.Bytes(str)
	if err != nil {
		return "", fmt.Errorf("decoding UTF-16 error: %v", err)
	}
	return string(s), nil
}
