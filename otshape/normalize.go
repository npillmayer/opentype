package otshape

import (
	"sort"
	"unicode"

	"github.com/npillmayer/opentype/ot"
	"golang.org/x/text/unicode/norm"
)

func normalizeRuneStream(
	runes []rune,
	clusters []uint32,
	font *ot.Font,
	ctx SelectionContext,
	engine ShapingEngine,
	pl *plan,
) ([]rune, []uint32) {
	runes, clusters, _, _, _, _ = normalizeRuneStreamWithScratch(
		runes,
		clusters,
		font,
		ctx,
		engine,
		pl,
		nil,
		nil,
		nil,
		nil,
	)
	return runes, clusters
}

func normalizeRuneStreamWithScratch(
	runes []rune,
	clusters []uint32,
	font *ot.Font,
	ctx SelectionContext,
	engine ShapingEngine,
	pl *plan,
	tmpARunes []rune,
	tmpAClusters []uint32,
	tmpBRunes []rune,
	tmpBClusters []uint32,
) ([]rune, []uint32, []rune, []uint32, []rune, []uint32) {
	if len(runes) == 0 {
		return runes, clusters, tmpARunes, tmpAClusters, tmpBRunes, tmpBClusters
	}
	mode := NormalizationAuto
	if ep, ok := engine.(ShapingEnginePolicy); ok {
		mode = ep.NormalizationPreference()
	}
	if mode == NormalizationAuto {
		if prefersDecomposed(ctx.ScriptTag, ctx.LangTag) {
			mode = NormalizationDecomposed
		} else {
			mode = NormalizationComposed
		}
	}
	if mode == NormalizationNone {
		return runes, clusters, tmpARunes, tmpAClusters, tmpBRunes, tmpBClusters
	}

	nctx := newNormalizeContext(font, ctx, planHasGposMark(pl))
	runes, clusters = decomposeSupportedRunes(tmpARunes, tmpAClusters, runes, clusters, nctx, mode)
	tmpARunes, tmpAClusters = runes, clusters
	// Hebrew needs script-specific combining classes. Preserve its existing
	// order until the separate Hebrew ordering follow-up is addressed; applying
	// raw Unicode CCC here regresses the accepted shin/dot/qamats oracle.
	if ctx.ScriptTag != ot.T("hebr") && ctx.Script.String() != "Hebr" {
		orderCanonicalMarks(runes, clusters)
	}

	composeHook, hasComposeHook := engine.(ShapingEngineComposeHook)
	if !hasComposeHook && mode != NormalizationComposed {
		return runes, clusters, tmpARunes, tmpAClusters, tmpBRunes, tmpBClusters
	}
	runes, clusters = composeRuneStreamInto(
		tmpBRunes,
		tmpBClusters,
		runes,
		clusters,
		nctx,
		composeHook,
		hasComposeHook,
		mode == NormalizationComposed,
	)
	tmpBRunes, tmpBClusters = runes, clusters
	return runes, clusters, tmpARunes, tmpAClusters, tmpBRunes, tmpBClusters
}

// Decompose before ordering, including accents hidden inside composites.
// A supported standalone character can retain its spelling in composed mode.
// In a base/mark sequence prefer decomposition, but never replace a supported
// character with unavailable components. This is a shaping representation
// choice, not unconditional conversion of the input to Unicode NFD.
func decomposeSupportedRunes(
	outRunes []rune,
	outClusters []uint32,
	runes []rune,
	clusters []uint32,
	nctx normalizeContext,
	mode NormalizationMode,
) ([]rune, []uint32) {
	outRunes = outRunes[:0]
	outClusters = outClusters[:0]
	for i, r := range runes {
		var cluster uint32
		if len(clusters) == len(runes) {
			cluster = clusters[i]
		} else {
			cluster = uint32(i)
		}
		shortest := mode == NormalizationComposed &&
			!unicode.Is(unicode.M, r) && (i+1 == len(runes) || !unicode.Is(unicode.M, runes[i+1]))
		decomposed := supportedDecomposition(r, nctx, shortest)
		for _, dr := range decomposed {
			outRunes = append(outRunes, dr)
			outClusters = append(outClusters, cluster)
		}
	}
	return outRunes, outClusters
}

func supportedDecomposition(r rune, nctx normalizeContext, shortest bool) []rune {
	if shortest && nctx.hasGlyph(r) {
		return []rune{r}
	}
	decomposed := []rune(norm.NFD.String(string(r)))
	// Try a supported nucleus followed by supported marks. A partially
	// composed nucleus matters when the font lacks a fully decomposed component
	// (e.g. A-ring-acute can still use A-ring + acute without a ring glyph).
	start, end, step := 1, len(decomposed)+1, 1
	if shortest {
		start, end, step = len(decomposed), 0, -1
	}
	for split := start; split != end; split += step {
		prefix := []rune(norm.NFC.String(string(decomposed[:split])))
		if len(prefix) != 1 || !nctx.hasGlyph(prefix[0]) {
			continue
		}
		available := true
		for _, mark := range decomposed[split:] {
			if !nctx.hasGlyph(mark) {
				available = false
				break
			}
		}
		if available {
			return append(prefix, decomposed[split:]...)
		}
	}
	return []rune{r}
}

func canonicalClass(r rune) uint8 {
	return norm.NFD.PropertiesString(string(r)).CCC()
}

// Sort only nonzero-class spans, stably. Starters (including CGJ) are barriers;
// equal-class marks retain their input order. Sources move with their runes.
func orderCanonicalMarks(runes []rune, sources []uint32) {
	for start := 0; start < len(runes); {
		if canonicalClass(runes[start]) == 0 {
			start++
			continue
		}
		end := start + 1
		for end < len(runes) && canonicalClass(runes[end]) != 0 {
			end++
		}
		sort.Stable(canonicalSequence{runes[start:end], sources[start:end]})
		start = end
	}
}

type canonicalSequence struct {
	runes   []rune
	sources []uint32
}

func (s canonicalSequence) Len() int { return len(s.runes) }
func (s canonicalSequence) Less(i, j int) bool {
	return canonicalClass(s.runes[i]) < canonicalClass(s.runes[j])
}
func (s canonicalSequence) Swap(i, j int) {
	s.runes[i], s.runes[j] = s.runes[j], s.runes[i]
	s.sources[i], s.sources[j] = s.sources[j], s.sources[i]
}

func composeRuneStream(
	runes []rune,
	clusters []uint32,
	nctx normalizeContext,
	hook ShapingEngineComposeHook,
	hasHook bool,
	allowUnicode bool,
) ([]rune, []uint32) {
	return composeRuneStreamInto(nil, nil, runes, clusters, nctx, hook, hasHook, allowUnicode)
}

func composeRuneStreamInto(
	outRunes []rune,
	outClusters []uint32,
	runes []rune,
	clusters []uint32,
	nctx normalizeContext,
	hook ShapingEngineComposeHook,
	hasHook bool,
	allowUnicode bool,
) ([]rune, []uint32) {
	if len(runes) <= 1 {
		// The workspace retains these slices as reusable scratch. Never lend it
		// raw input storage: the next event span could overwrite unread input.
		return append(outRunes[:0], runes...), append(outClusters[:0], clusters...)
	}
	outRunes = outRunes[:0]
	outClusters = outClusters[:0]
	starter := 0
	for i, r := range runes {
		cluster := uint32(i)
		if len(clusters) == len(runes) {
			cluster = clusters[i]
		}
		if len(outRunes) == 0 {
			outRunes = append(outRunes, r)
			outClusters = append(outClusters, cluster)
			continue
		}
		last := len(outRunes) - 1
		ccc := canonicalClass(r)
		// An unconsumed intervening mark with equal or higher class blocks
		// composition. Adjacent pairs retain the existing custom-hook contract,
		// including hooks that deliberately compose two class-zero characters.
		if starter == last || canonicalClass(outRunes[last]) < ccc {
			a := outRunes[starter]
			var composed rune
			var ok bool
			if hasHook {
				composed, ok = hook.Compose(nctx, a, r)
				ok = ok && nctx.hasGlyph(composed)
			}
			if !ok && allowUnicode {
				composed, ok = nctx.ComposeUnicode(a, r)
			}
			if ok {
				outRunes[starter] = composed
				outClusters[starter] = min(outClusters[starter], cluster)
				continue
			}
		}
		if ccc == 0 {
			starter = len(outRunes)
		}
		outRunes = append(outRunes, r)
		outClusters = append(outClusters, cluster)
	}
	return outRunes, outClusters
}

func planHasGposMark(pl *plan) bool {
	if pl == nil || !pl.Policy.ApplyGPOS {
		return false
	}
	// Normalization asks about plan-wide mark support, not the mask at a
	// particular input position. Only resolved mark lookup contributors count:
	// mkmk alone cannot replace the Hebrew presentation-form fallback.
	mark := ot.T("mark")
	for _, op := range pl.GPOS.Lookups {
		for _, feature := range op.Features {
			if feature.Tag != mark {
				continue
			}
			// Defaults and required features can run without an allocated mask.
			// Use the contributor's mask, not the shared lookup's combined mask.
			if feature.Mask == 0 || pl.Masks.GlobalMask&feature.Mask != 0 {
				return true
			}
			for _, r := range pl.featureRanges {
				if r.Feature == mark && r.On && (r.Start != 0 || r.End != 0) {
					return true
				}
			}
		}
	}
	return false
}
