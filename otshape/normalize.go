package otshape

import (
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

	if mode == NormalizationDecomposed {
		runes, clusters = decomposeRuneStreamInto(tmpARunes, tmpAClusters, runes, clusters)
		tmpARunes, tmpAClusters = runes, clusters
	}

	composeHook, hasComposeHook := engine.(ShapingEngineComposeHook)
	if !hasComposeHook && mode != NormalizationComposed {
		return runes, clusters, tmpARunes, tmpAClusters, tmpBRunes, tmpBClusters
	}
	nctx := newNormalizeContext(font, ctx, planHasGposMark(pl))
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

func decomposeRuneStream(runes []rune, clusters []uint32) ([]rune, []uint32) {
	return decomposeRuneStreamInto(nil, nil, runes, clusters)
}

func decomposeRuneStreamInto(
	outRunes []rune,
	outClusters []uint32,
	runes []rune,
	clusters []uint32,
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
		s := norm.NFD.String(string(r))
		for _, dr := range s {
			outRunes = append(outRunes, dr)
			outClusters = append(outClusters, cluster)
		}
	}
	return outRunes, outClusters
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
		a := outRunes[last]
		if hasHook {
			if composed, ok := hook.Compose(nctx, a, r); ok {
				outRunes[last] = composed
				if cluster < outClusters[last] {
					outClusters[last] = cluster
				}
				continue
			}
		}
		if allowUnicode {
			if composed, ok := nctx.ComposeUnicode(a, r); ok {
				outRunes[last] = composed
				if cluster < outClusters[last] {
					outClusters[last] = cluster
				}
				continue
			}
		}
		outRunes = append(outRunes, r)
		outClusters = append(outClusters, cluster)
	}
	return outRunes, outClusters
}

func planHasGposMark(pl *plan) bool {
	if pl == nil {
		return false
	}
	if ms, ok := pl.maskForFeature(ot.T("mark")); ok && ms.Mask != 0 {
		return true
	}
	if ms, ok := pl.maskForFeature(ot.T("mkmk")); ok && ms.Mask != 0 {
		return true
	}
	return false
}
