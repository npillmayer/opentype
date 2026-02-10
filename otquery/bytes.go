package otquery

func u16(b []byte) uint16 {
	return uint16(b[0])<<8 | uint16(b[1])<<0
}

func i16(b []byte) int16 {
	return int16(b[0])<<8 | int16(b[1])<<0
}
