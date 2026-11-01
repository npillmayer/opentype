package otshape

import (
	"errors"
	"fmt"

	"github.com/npillmayer/opentype/ot"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

type planTable uint8

const (
	planGSUB planTable = iota
	planGPOS
)

func (t planTable) String() string {
	assert(t == planGSUB || t == planGPOS, "plan table is neither GSUB nor GPOS")
	switch t {
	case planGSUB:
		return "GSUB"
	case planGPOS:
		return "GPOS"
	}
	panic("unreachable")
}

type lookupRunFlags uint8

const (
	lookupAutoZWNJ lookupRunFlags = 1 << iota
	lookupAutoZWJ
	lookupRandom
	lookupPerSyllable
)

func (f lookupRunFlags) has(wanted lookupRunFlags) bool {
	return f&wanted == wanted
}

type pauseHookID uint16

const noPauseHook pauseHookID = 0

type pauseHook func(run *runBuffer) error

type segmentProps struct {
	Direction bidi.Direction
	Script    language.Script
	Language  language.Tag
}

type maskSpec struct {
	Mask         uint32
	Shift        uint8
	DefaultValue uint32
	Values       []int // mask codes: zero is off, one is default; remaining codes retain requested values
}

type maskLayout struct {
	GlobalMask uint32
	ByFeature  map[ot.Tag]maskSpec
}

type featureBind struct {
	Tag          ot.Tag
	FeatureIndex uint16
	Stage        int
	Mask         uint32
	Required     bool
}

type lookupOp struct {
	LookupIndex uint16
	FeatureTag  ot.Tag
	Mask        uint32
	Flags       lookupRunFlags
}

type stage struct {
	FirstLookup int // inclusive
	LastLookup  int // exclusive
	Pause       pauseHookID
}

type tableProgram struct {
	FoundScript  bool
	Stages       []stage
	Lookups      []lookupOp
	FeatureBinds []featureBind
}

func (tp tableProgram) lookupCount() int {
	return len(tp.Lookups)
}

func (tp tableProgram) stageCount() int {
	return len(tp.Stages)
}

func (tp tableProgram) lookupsForStage(stageIndex int) []lookupOp {
	if stageIndex < 0 || stageIndex >= len(tp.Stages) {
		return nil
	}
	st := tp.Stages[stageIndex]
	if st.FirstLookup < 0 || st.FirstLookup > st.LastLookup || st.LastLookup > len(tp.Lookups) {
		return nil
	}
	out := make([]lookupOp, st.LastLookup-st.FirstLookup)
	copy(out, tp.Lookups[st.FirstLookup:st.LastLookup])
	return out
}

func (tp tableProgram) validate(maxPauseID pauseHookID) error {
	last := 0
	for i, st := range tp.Stages {
		if st.FirstLookup < 0 || st.LastLookup < st.FirstLookup || st.LastLookup > len(tp.Lookups) {
			return fmt.Errorf("invalid stage %d lookup bounds [%d:%d) for %d lookups",
				i, st.FirstLookup, st.LastLookup, len(tp.Lookups))
		}
		if i > 0 && st.FirstLookup < last {
			return fmt.Errorf("stage %d starts before previous stage end", i)
		}
		last = st.LastLookup
		if st.Pause > maxPauseID {
			return fmt.Errorf("stage %d references unknown pause hook id %d", i, st.Pause)
		}
	}
	return nil
}

type planPolicy struct {
	Strict        bool // fail early on unsupported/incomplete OT data
	ApplyGPOS     bool // run GPOS stage at execution time
	ZeroGDEFMarks bool // cancel final advances of GDEF marks after GPOS (Latin/Hebrew)
}

type planHookSet struct {
	pause []pauseHook
}

func newPlanHookSet() planHookSet {
	return planHookSet{pause: []pauseHook{nil}} // reserve id 0 (noPauseHook)
}

func (hs *planHookSet) addPause(fn pauseHook) pauseHookID {
	if hs == nil || fn == nil {
		return noPauseHook
	}
	if len(hs.pause) == 0 {
		hs.pause = append(hs.pause, nil)
	}
	hs.pause = append(hs.pause, fn)
	return pauseHookID(len(hs.pause) - 1)
}

func (hs planHookSet) pauseHook(id pauseHookID) (pauseHook, bool) {
	if id == noPauseHook || int(id) >= len(hs.pause) {
		return nil, false
	}
	fn := hs.pause[id]
	return fn, fn != nil
}

func (hs planHookSet) maxPauseID() pauseHookID {
	if len(hs.pause) == 0 {
		return noPauseHook
	}
	return pauseHookID(len(hs.pause) - 1)
}

func (hs planHookSet) clone() planHookSet {
	out := planHookSet{pause: make([]pauseHook, len(hs.pause))}
	copy(out.pause, hs.pause)
	return out
}

type planNoteLevel uint8

const (
	planNoteInfo planNoteLevel = iota
	planNoteWarning
)

type planNote struct {
	Level   planNoteLevel
	Message string
}

type plan struct {
	randomSeed uint64 // shared by all plans of one shaping request
	font       *ot.Font

	Props     segmentProps
	ScriptTag ot.Tag
	LangTag   ot.Tag
	VarIndex  [2]int // GSUB, GPOS variation selection (-1 if none)

	Masks maskLayout
	GSUB  tableProgram
	GPOS  tableProgram

	Policy planPolicy
	Hooks  planHookSet
	Notes  []planNote

	featureRanges    []FeatureRange          // preserved for runtime mask setup
	joinerGlyphClass map[ot.GlyphIndex]uint8 // GSUB-time joiner annotation by glyph
}

func (p *plan) table(t planTable) *tableProgram {
	assert(t == planGPOS || t == planGSUB, "invalid table type")
	if t == planGPOS {
		return &p.GPOS
	}
	return &p.GSUB
}

func (p *plan) maskForFeature(tag ot.Tag) (maskSpec, bool) {
	if p == nil || p.Masks.ByFeature == nil {
		return maskSpec{}, false
	}
	ms, ok := p.Masks.ByFeature[tag]
	return ms, ok
}

func (p *plan) validate() error {
	if p == nil {
		return errors.New("plan is nil")
	}
	if p.Masks.ByFeature == nil {
		return errors.New("plan masks are uninitialized")
	}
	maxPauseID := p.Hooks.maxPauseID()
	if err := p.GSUB.validate(maxPauseID); err != nil {
		return fmt.Errorf("GSUB program invalid: %w", err)
	}
	if err := p.GPOS.validate(maxPauseID); err != nil {
		return fmt.Errorf("GPOS program invalid: %w", err)
	}
	return nil
}

// featureValue canonicalizes equivalent requests without narrowing int values.
func featureValue(value int) int {
	if value < 0 {
		return -1
	}
	if value == 0 {
		return 1
	}
	return value
}

func (s maskSpec) encode(value int) uint32 {
	value = featureValue(value)
	if s.Values == nil {
		// Engine-only and manually constructed internal layouts use raw bits.
		return uint32(value)
	}
	for code, v := range s.Values {
		if code != 0 && v == value {
			return uint32(code)
		}
	}
	panic("feature value absent from compiled mask layout")
}

func (s maskSpec) decode(mask uint32) int {
	code := (mask & s.Mask) >> s.Shift
	if s.Values == nil {
		return int(code)
	}
	if int(code) >= len(s.Values) {
		return 0
	}
	return s.Values[code]
}
