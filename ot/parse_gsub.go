package ot

// parseGSub parses the GSUB (Glyph Substitution) table.
func parseGSub(tag Tag, b binarySegm, offset, size uint32, ec *errorCollector) (Table, error) {
	var err error
	gsub := newGSubTable(tag, b, offset, size)
	err = parseLayoutHeader(&gsub.LayoutTable, b, err, tag, ec)
	err = parseLookupList(&gsub.LayoutTable, b, err, false, tag, ec) // false = GSUB
	err = parseFeatureList(&gsub.LayoutTable, b, err)
	err = parseScriptList(&gsub.LayoutTable, b, err)
	if err != nil {
		tracer().Errorf("error parsing GSUB table: %v", err)
		return gsub, err
	}
	mj, mn := gsub.header.Version()
	tracer().Debugf("GSUB table has version %d.%d", mj, mn)
	if graph := gsub.LookupGraph(); graph != nil {
		tracer().Debugf("GSUB table has %d lookup list entries", graph.Len())
	}
	return gsub, err
}
