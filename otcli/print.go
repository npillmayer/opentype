package main

import (
	"fmt"
	"strings"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"github.com/pterm/pterm"
)

func printOp(intp *Intp, op *Op) (err error, stop bool) {
	pterm.Printf("PRINT %s\n", opNames[op.code])
	var nav ot.Navigator
	if nav, err = intp.checkLocation(); err != nil {
		return
	}
	pterm.Printf("Current location: %s\n", nav.Name())
	n := intp.lastPathNode()
	sb := strings.Builder{}
	if n.key != "" {
		sb.WriteString(fmt.Sprintf("%s[%s]", nav.Name(), n.key))
	} else if n.inx >= 0 {
		sb.WriteString(fmt.Sprintf("%s[%d]", nav.Name(), n.inx))
	} else {
		if t, err2 := otlayout.NavAsTagRecordMap(nav); err2 == nil {
			sb.WriteString(fmt.Sprintf("%s@%v", nav.Name(), t.Tags()))
		} else if l, err2 := otlayout.NavAsList(nav); err2 == nil {
			sb.WriteString(fmt.Sprintf("%s|%d|", nav.Name(), l.Len()))
		} else {
			sb.WriteString(nav.Name())
		}
	}
	if n.link != nil {
		sb.WriteString(fmt.Sprintf(" -> (%s)", n.link.Name()))
	}
	pterm.Printf("Current location: %s\n", sb.String())
	return nil, false
}

func printLookupList(table *ot.LayoutTable) {
	if table == nil {
		pterm.Error.Println("GSUB table is nil")
		return
	}
	ll := table.LookupList
	count := ll.Len()
	pterm.Printf("GSUB LookupList has %d entries\n", count)
	if count == 0 {
		return
	}
	data := [][]string{
		{"Index", "Type", "Subtables", "Flags"},
	}
	for i := range count {
		lookup := ll.Navigate(i)
		data = append(data, []string{
			fmt.Sprintf("%d", i),
			formatLookupType(lookup.Type),
			fmt.Sprintf("%d", lookup.SubTableCount),
			formatLookupFlags(lookup.Flag),
		})
	}
	pterm.DefaultTable.WithHasHeader().WithData(data).Render()
}

func printLookup(table *ot.LayoutTable, index int) {
	if table == nil {
		pterm.Error.Println("GSUB table is nil")
		return
	}
	ll := table.LookupList
	if index < 0 || index >= ll.Len() {
		pterm.Error.Printf("Lookup index out of range: %d\n", index)
		return
	}
	lookup := ll.Navigate(index)
	pterm.Printf("Lookup %d: type=%s flags=%s subtables=%d\n",
		index,
		formatLookupType(lookup.Type),
		formatLookupFlags(lookup.Flag),
		lookup.SubTableCount,
	)
	data := [][]string{
		{"Sub", "Type", "Format", "Coverage", "Index", "Support"},
	}
	for i := 0; i < int(lookup.SubTableCount); i++ {
		sub := lookup.Subtable(i)
		if sub == nil {
			continue
		}
		data = append(data, []string{
			fmt.Sprintf("%d", i),
			formatLookupType(sub.LookupType),
			fmt.Sprintf("%d", sub.Format),
			formatCoverageSummary(sub),
			formatVarArraySummary(sub.Index),
			formatSupportSummary(sub.Support),
		})
	}
	pterm.DefaultTable.WithHasHeader().WithData(data).Render()
}

func formatLookupType(ltype ot.LayoutTableLookupType) string {
	if ltype == 0 {
		return "Unknown(0)"
	}
	if ot.IsGPosLookupType(ltype) {
		unmasked := ot.GPosLookupType(ltype)
		if unmasked == 0 {
			return "Unknown(0)"
		}
		return unmasked.GPosString()
	}
	return ltype.GSubString()
}

func formatLookupFlags(flag ot.LayoutTableLookupFlag) string {
	if flag == 0 {
		return "-"
	}
	parts := make([]string, 0, 6)
	if flag&ot.LOOKUP_FLAG_RIGHT_TO_LEFT != 0 {
		parts = append(parts, "RightToLeft")
	}
	if flag&ot.LOOKUP_FLAG_IGNORE_BASE_GLYPHS != 0 {
		parts = append(parts, "IgnoreBase")
	}
	if flag&ot.LOOKUP_FLAG_IGNORE_LIGATURES != 0 {
		parts = append(parts, "IgnoreLigatures")
	}
	if flag&ot.LOOKUP_FLAG_IGNORE_MARKS != 0 {
		parts = append(parts, "IgnoreMarks")
	}
	if flag&ot.LOOKUP_FLAG_USE_MARK_FILTERING_SET != 0 {
		parts = append(parts, "UseMarkFilteringSet")
	}
	if flag&ot.LOOKUP_FLAG_MARK_ATTACHMENT_TYPE_MASK != 0 {
		parts = append(parts, fmt.Sprintf("MarkAttachType=%d", flag>>8))
	}
	return strings.Join(parts, "|")
}

func formatCoverageSummary(sub *ot.LookupSubtable) string {
	if sub == nil || sub.Coverage.GlyphRange == nil {
		return "-"
	}
	return fmt.Sprintf("fmt=%d count=%d bytes=%d",
		sub.Coverage.CoverageFormat,
		sub.Coverage.Count,
		sub.Coverage.GlyphRange.ByteSize(),
	)
}

func formatVarArraySummary(index ot.VarArray) string {
	if index == nil {
		return "-"
	}
	return fmt.Sprintf("size=%d", index.Size())
}

func formatSupportSummary(support any) string {
	if support == nil {
		return "-"
	}
	switch v := support.(type) {
	case int16:
		return fmt.Sprintf("delta=%d", v)
	case ot.GlyphIndex:
		return fmt.Sprintf("delta=%d", v)
	case ot.SequenceContext:
		return formatSequenceContextSummary(&v)
	case *ot.SequenceContext:
		return formatSequenceContextSummary(v)
	default:
		return fmt.Sprintf("%T", support)
	}
}

func formatSequenceContextSummary(ctx *ot.SequenceContext) string {
	if ctx == nil {
		return "-"
	}
	return fmt.Sprintf("seqctx back=%d in=%d look=%d class=%d",
		len(ctx.BacktrackCoverage),
		len(ctx.InputCoverage),
		len(ctx.LookaheadCoverage),
		len(ctx.ClassDefs),
	)
}
