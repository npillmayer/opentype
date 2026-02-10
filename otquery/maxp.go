package otquery

import (
	"encoding/binary"

	"github.com/npillmayer/opentype/ot"
)

// MaxPTableInfo is a typed query view over OpenType table 'maxp'.
// For version 1.0 tables, extended profile fields are decoded if present.
type MaxPTableInfo struct {
	VersionFixed uint32
	NumGlyphs    uint16

	// TrueType profile fields (version 1.0 only)
	HasExtendedProfile bool
	MaxPoints          uint16
	MaxContours        uint16
	MaxCompositePoints uint16
	MaxCompositeContours uint16
	MaxZones             uint16
	MaxTwilightPoints    uint16
	MaxStorage           uint16
	MaxFunctionDefs      uint16
	MaxInstructionDefs   uint16
	MaxStackElements     uint16
	MaxSizeOfInstructions uint16
	MaxComponentElements  uint16
	MaxComponentDepth     uint16
}

const maxpMinSize = 6
const maxpV10Size = 32

// MaxPInfo decodes table 'maxp' directly from raw bytes without using Navigator/Fields.
// Returns (info, true) on success, or (zero, false) if table is missing/too short.
func MaxPInfo(otf *ot.Font) (MaxPTableInfo, bool) {
	var info MaxPTableInfo
	if otf == nil {
		return info, false
	}
	table := otf.Table(ot.T("maxp"))
	if table == nil {
		return info, false
	}
	b := table.Binary()
	if len(b) < maxpMinSize {
		return info, false
	}
	info.VersionFixed = binary.BigEndian.Uint32(b[0:4])
	info.NumGlyphs = binary.BigEndian.Uint16(b[4:6])

	if info.VersionFixed != 0x00010000 || len(b) < maxpV10Size {
		return info, true
	}
	info.HasExtendedProfile = true
	info.MaxPoints = binary.BigEndian.Uint16(b[6:8])
	info.MaxContours = binary.BigEndian.Uint16(b[8:10])
	info.MaxCompositePoints = binary.BigEndian.Uint16(b[10:12])
	info.MaxCompositeContours = binary.BigEndian.Uint16(b[12:14])
	info.MaxZones = binary.BigEndian.Uint16(b[14:16])
	info.MaxTwilightPoints = binary.BigEndian.Uint16(b[16:18])
	info.MaxStorage = binary.BigEndian.Uint16(b[18:20])
	info.MaxFunctionDefs = binary.BigEndian.Uint16(b[20:22])
	info.MaxInstructionDefs = binary.BigEndian.Uint16(b[22:24])
	info.MaxStackElements = binary.BigEndian.Uint16(b[24:26])
	info.MaxSizeOfInstructions = binary.BigEndian.Uint16(b[26:28])
	info.MaxComponentElements = binary.BigEndian.Uint16(b[28:30])
	info.MaxComponentDepth = binary.BigEndian.Uint16(b[30:32])
	return info, true
}
