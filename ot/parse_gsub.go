package ot

// parseGSub parses the GSUB (Glyph Substitution) table.
func parseGSub(tag Tag, b binarySegm, offset, size uint32) (Table, error) {
	var err error
	gsub := newGSubTable(tag, b, offset, size)
	err = parseLayoutHeader(&gsub.LayoutTable, b, err)
	err = parseLookupList(&gsub.LayoutTable, b, err, false) // false = GSUB
	err = parseFeatureList(&gsub.LayoutTable, b, err)
	err = parseScriptList(&gsub.LayoutTable, b, err)
	if err != nil {
		tracer().Errorf("error parsing GSUB table: %v", err)
		return gsub, err
	}
	mj, mn := gsub.header.Version()
	tracer().Debugf("GSUB table has version %d.%d", mj, mn)
	tracer().Debugf("GSUB table has %d lookup list entries", gsub.LookupList.length)
	return gsub, err
}

// parseGSubLookupSubtable parses a segment of binary data from a font file (NavLocation)
// and expects to read a lookup subtable.
func parseGSubLookupSubtable(b binarySegm, lookupType LayoutTableLookupType) LookupSubtable {
	return parseGSubLookupSubtableWithDepth(b, lookupType, 0)
}

func parseGSubLookupSubtableWithDepth(b binarySegm, lookupType LayoutTableLookupType, depth int) LookupSubtable {
	// Validate minimum buffer size to prevent panics
	if len(b) < 4 {
		tracer().Errorf("GSUB lookup subtable buffer too small: %d bytes", len(b))
		return LookupSubtable{}
	}

	format := b.U16(0)
	tracer().Debugf("parsing GSUB sub-table type %s, format %d at depth %d", lookupType.GSubString(), format, depth)
	sub := LookupSubtable{LookupType: lookupType, Format: format}
	// Most of the subtable formats use a coverage table in some form to decide on which glyphs to
	// operate on. parseGSubLookupSubtable will parse this coverage table and put it into
	// `sub.Coverage`, then branch down to the different lookup types.
	if !(lookupType == 7 && format == 3) { // GSUB type Extension has no coverage table
		covlink, err := parseLink16(b, 2, b, "Coverage")
		if err == nil {
			sub.Coverage = parseCoverage(covlink.Jump().Bytes())
		}
	}
	switch lookupType {
	case 1:
		return parseGSubLookupSubtableType1(b, sub)
	case 2, 3, 4:
		return parseGSubLookupSubtableType2or3or4(b, sub)
	case 5:
		return parseGSubLookupSubtableType5(b, sub)
	case 6:
		return parseGSubLookupSubtableType6(b, sub)
	case 7:
		return parseGSubLookupSubtableType7WithDepth(b, sub, depth)
	}
	tracer().Errorf("unknown GSUB lookup type: %d", lookupType)
	return LookupSubtable{}
}

// LookupType 1: Single Substitution Subtable
// Single substitution (SingleSubst) subtables tell a client to replace a single glyph with
// another glyph.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#lookuptype-1-single-substitution-subtable
func parseGSubLookupSubtableType1(b binarySegm, sub LookupSubtable) LookupSubtable {
	if len(b) < 6 {
		tracer().Errorf("GSUB type 1 buffer too small: %d bytes", len(b))
		return LookupSubtable{}
	}

	if sub.Format == 1 {
		sub.Support = int16(b.U16(4))
	} else {
		sub.Index = parseVarArray16(b, 4, 2, 1, "LookupSubtableGSub1")
	}
	return sub
}

// LookupType 2: Multiple Substitution Subtable
// A Multiple Substitution (MultipleSubst) subtable replaces a single glyph with more than
// one glyph, as when multiple glyphs replace a single ligature.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#lookuptype-2-multiple-substitution-subtable
//
// LookupType 3: Alternate Substitution Subtable
// An Alternate Substitution (AlternateSubst) subtable identifies any number of aesthetic
// alternatives from which a user can choose a glyph variant to replace the input glyph.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#lookuptype-3-alternate-substitution-subtable
//
// LookupType 4: Ligature Substitution Subtable
// A Ligature Substitution (LigatureSubst) subtable identifies ligature substitutions where
// a single glyph replaces multiple glyphs. One LigatureSubst subtable can specify any
// number of ligature substitutions.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#lookuptype-4-ligature-substitution-subtable
func parseGSubLookupSubtableType2or3or4(b binarySegm, sub LookupSubtable) LookupSubtable {
	sub.Index = parseVarArray16(b, 4, 2, 2, "LookupSubtableGSub2/3/4")
	return sub
}

// LookupType 5: Contextual Substitution Subtable
// A Contextual Substitution subtable describes glyph substitutions in context that replace one or more
// glyphs within a certain pattern of glyphs.
// Input sequence patterns are matched against the text glyph sequence, and then actions to be applied
// to glyphs within the input sequence. The actions are specified as "nested" lookups, and each is applied
// to a particular sequence position within the input sequence.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#lookuptype-5-contextual-substitution-subtable
//
// For contextual substitution subtables we usually will have to parse a rule set. We will put it
// into the Index field. Additional context data structures may include ClassDefs or other things and
// will be put into the Support field by calling parseSequenceContext.
func parseGSubLookupSubtableType5(b binarySegm, sub LookupSubtable) LookupSubtable {
	switch sub.Format {
	case 1:
		sub.Index = parseVarArray16(b, 4, 2, 2, "LookupSubtableGSub5-1")
	case 2:
		sub.Index = parseVarArray16(b, 6, 2, 2, "LookupSubtableGSub5-2")
	case 3:
		sub.Index = parseVarArray16(b, 4, 4, 2, "LookupSubtableGSub5-3")
	}
	var err error
	sub, err = parseSequenceContext(b, sub)
	if err != nil {
		tracer().Errorf(err.Error()) // nothing we can/will do about it
	}
	return sub
}

// LookupType 6: Chained Contexts Substitution Subtable
// A Chained Contexts Substitution subtable describes glyph substitutions in context with an ability to
// look back and/or look ahead in the sequence of glyphs. The design of the Chained Contexts Substitution
// subtable is parallel to that of the Contextual Substitution subtable, including the availability of
// three formats. Each format can describe one or more chained backtrack, input, and lookahead sequence
// combinations, and one or more substitutions for glyphs in each input sequence.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#chained-sequence-context-format-1-simple-glyph-contexts
func parseGSubLookupSubtableType6(b binarySegm, sub LookupSubtable) LookupSubtable {
	if len(b) < 6 {
		tracer().Errorf("GSUB type 6 buffer too small: %d bytes", len(b))
		return LookupSubtable{}
	}

	var err error
	sub, err = parseChainedSequenceContext(b, sub)
	if err != nil {
		tracer().Errorf("GSUB type 6 chained context error: %v", err)
		return LookupSubtable{}
	}

	switch sub.Format {
	case 1:
		sub.Index = parseVarArray16(b, 4, 2, 2, "LookupSubtableGSub6-1")
	case 2:
		if len(b) < 12 {
			tracer().Errorf("GSUB type 6 format 2 buffer too small: %d bytes", len(b))
			return LookupSubtable{}
		}
		sub.Index = parseVarArray16(b, 10, 2, 2, "LookupSubtableGSub6-2")
	case 3:
		// Safe type assertion to prevent panic
		seqctx, ok := sub.Support.(*SequenceContext)
		if !ok {
			tracer().Errorf("GSUB type 6 format 3: Support is not *SequenceContext")
			return LookupSubtable{}
		}

		offset := 2 // skip over format field
		offset += 2 + len(seqctx.BacktrackCoverage)*2
		offset += 2 + len(seqctx.InputCoverage)*2
		offset += 2 + len(seqctx.LookaheadCoverage)*2

		if offset >= len(b) {
			tracer().Errorf("GSUB type 6 format 3: offset %d exceeds buffer size %d", offset, len(b))
			return LookupSubtable{}
		}

		sub.Index = parseVarArray16(b, offset, 2, 2, "LookupSubtableGSub6-3")
	}
	return sub
}

// LookupType 7: Extension Substitution
// This lookup provides a mechanism whereby any other lookup type's subtables are stored at
// a 32-bit offset location in the GSUB table.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#lookuptype-7-extension-substitution
func parseGSubLookupSubtableType7(b binarySegm, sub LookupSubtable) LookupSubtable {
	return parseGSubLookupSubtableType7WithDepth(b, sub, 0)
}

func parseGSubLookupSubtableType7WithDepth(b binarySegm, sub LookupSubtable, depth int) LookupSubtable {
	if b.Size() < 8 {
		tracer().Errorf("OpenType GSUB lookup subtable type %d corrupt", sub.LookupType)
		return LookupSubtable{}
	}

	actualType := LayoutTableLookupType(b.U16(2))
	if actualType == GSubLookupTypeExtensionSubs {
		tracer().Errorf("OpenType GSUB extension subtable cannot recursively reference extension type")
		return LookupSubtable{}
	}

	tracer().Debugf("OpenType GSUB extension subtable is of type %s at depth %d", actualType.GSubString(), depth)
	link, _ := parseLink32(b, 4, b, "ext.LookupSubtable")
	loc := link.Jump()

	// Recurse with incremented depth
	return parseGSubLookupSubtableWithDepth(loc.Bytes(), actualType, depth+1)
}
