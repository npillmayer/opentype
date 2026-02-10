package otquery

import (
	"fmt"

	"github.com/npillmayer/opentype/ot"
	"golang.org/x/text/encoding/unicode"
)

const (
	nameHeaderSize = 6
	nameRecordSize = 12
)

type nameEntry struct {
	key   nameKey
	value string
}

func loadNameEntries(otf *ot.Font) []nameEntry {
	if otf == nil {
		return nil
	}
	table := otf.Table(ot.T("name"))
	if table == nil {
		tracer().Debugf("no name table found in font %s", otf.F.Fontname)
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
	entries := make([]nameEntry, 0, count)
	for i := 0; i < count; i++ {
		rec := b[nameHeaderSize+i*nameRecordSize : nameHeaderSize+(i+1)*nameRecordSize]
		key := nameKey{
			PlatformID: u16(rec[0:2]),
			EncodingID: u16(rec[2:4]),
			LanguageID: u16(rec[4:6]),
			NameID:     u16(rec[6:8]),
		}
		if !isSupportedNameEncoding(key) {
			continue
		}
		strLen := int(u16(rec[8:10]))
		recOff := int(u16(rec[10:12]))
		start := strOff + recOff
		end := start + strLen
		if start < 0 || strLen < 0 || end > len(b) {
			continue
		}
		val, err := decodeNameUTF16(b[start:end])
		if err != nil || val == "" {
			continue
		}
		entries = append(entries, nameEntry{key: key, value: val})
	}
	return entries
}

func isSupportedNameEncoding(key nameKey) bool {
	// Keep current behavior: decode Unicode BMP + Windows BMP entries only.
	return (key.PlatformID == 0 && key.EncodingID == 3) || (key.PlatformID == 3 && key.EncodingID == 1)
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
