package otquery

import (
	"encoding/binary"

	"github.com/npillmayer/opentype/ot"
)

// HeadTableInfo is a typed query view over OpenType table 'head'.
// Values are decoded directly from the raw table bytes.
type HeadTableInfo struct {
	MajorVersion       uint16
	MinorVersion       uint16
	FontRevision       uint32
	CheckSumAdjustment uint32
	MagicNumber        uint32
	Flags              uint16
	UnitsPerEm         uint16
	Created            int64
	Modified           int64
	XMin               int16
	YMin               int16
	XMax               int16
	YMax               int16
	MacStyle           uint16
	LowestRecPPEM      uint16
	FontDirectionHint  int16
	IndexToLocFormat   int16
	GlyphDataFormat    int16
}

const headTableSize = 54

// HeadInfo decodes table 'head' from raw bytes without relying on Navigator/Fields.
// Returns (info, true) on success, or (zero, false) if table is missing/too short.
func HeadInfo(otf *ot.Font) (HeadTableInfo, bool) {
	var info HeadTableInfo
	if otf == nil {
		return info, false
	}
	table := otf.Table(ot.T("head"))
	if table == nil {
		return info, false
	}
	b := table.Binary()
	if len(b) < headTableSize {
		return info, false
	}
	info.MajorVersion = binary.BigEndian.Uint16(b[0:2])
	info.MinorVersion = binary.BigEndian.Uint16(b[2:4])
	info.FontRevision = binary.BigEndian.Uint32(b[4:8])
	info.CheckSumAdjustment = binary.BigEndian.Uint32(b[8:12])
	info.MagicNumber = binary.BigEndian.Uint32(b[12:16])
	info.Flags = binary.BigEndian.Uint16(b[16:18])
	info.UnitsPerEm = binary.BigEndian.Uint16(b[18:20])
	info.Created = int64(binary.BigEndian.Uint64(b[20:28]))
	info.Modified = int64(binary.BigEndian.Uint64(b[28:36]))
	info.XMin = int16(binary.BigEndian.Uint16(b[36:38]))
	info.YMin = int16(binary.BigEndian.Uint16(b[38:40]))
	info.XMax = int16(binary.BigEndian.Uint16(b[40:42]))
	info.YMax = int16(binary.BigEndian.Uint16(b[42:44]))
	info.MacStyle = binary.BigEndian.Uint16(b[44:46])
	info.LowestRecPPEM = binary.BigEndian.Uint16(b[46:48])
	info.FontDirectionHint = int16(binary.BigEndian.Uint16(b[48:50]))
	info.IndexToLocFormat = int16(binary.BigEndian.Uint16(b[50:52]))
	info.GlyphDataFormat = int16(binary.BigEndian.Uint16(b[52:54]))
	return info, true
}
