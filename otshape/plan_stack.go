package otshape

import (
	"errors"
	"sort"

	"github.com/npillmayer/opentype/ot"
)

var (
	errPlanStackUnderflow = errors.New("otshape: feature-plan stack underflow")
	errPlanStackUnclosed  = errors.New("otshape: feature-plan stack not closed at end of stream")
)

type featureAssignment struct {
	enabled bool
	value   int
}

type featureSet struct {
	byTag map[ot.Tag]featureAssignment
}

func newFeatureSet(base []FeatureRange) featureSet {
	fs := featureSet{byTag: map[ot.Tag]featureAssignment{}}
	for _, f := range base {
		if f.Feature == 0 || !(f.Start == 0 && f.End == 0) {
			continue
		}
		value := f.Arg
		if f.On && value == 0 {
			value = 1
		}
		fs.byTag[f.Feature] = featureAssignment{
			enabled: f.On,
			value:   value,
		}
	}
	return fs
}

func (fs featureSet) clone() featureSet {
	out := featureSet{byTag: make(map[ot.Tag]featureAssignment, len(fs.byTag))}
	for tag, a := range fs.byTag {
		out.byTag[tag] = a
	}
	return out
}

func (fs featureSet) applyPush(settings []FeatureSetting) featureSet {
	next := fs.clone()
	for _, s := range settings {
		if s.Tag == 0 {
			continue
		}
		val := s.Value
		if s.Enabled && val == 0 {
			val = 1
		}
		next.byTag[s.Tag] = featureAssignment{
			enabled: s.Enabled,
			value:   val,
		}
	}
	return next
}

func (fs featureSet) asGlobalFeatureRanges() []FeatureRange {
	if len(fs.byTag) == 0 {
		return nil
	}
	tags := make([]ot.Tag, 0, len(fs.byTag))
	for tag := range fs.byTag {
		tags = append(tags, tag)
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i] < tags[j] })
	out := make([]FeatureRange, 0, len(tags))
	for _, tag := range tags {
		a := fs.byTag[tag]
		out = append(out, FeatureRange{
			Feature: tag,
			Arg:     a.value,
			On:      a.enabled,
		})
	}
	return out
}

type planFrame struct {
	id       uint16
	features featureSet
}

type planStack struct {
	frames []planFrame
	nextID uint16
}

func newPlanStack(rootFeatures []FeatureRange) *planStack {
	root := planFrame{
		id:       1,
		features: newFeatureSet(rootFeatures),
	}
	return &planStack{
		frames: []planFrame{root},
		nextID: 2,
	}
}

func (s *planStack) depth() int {
	if s == nil {
		return 0
	}
	return len(s.frames)
}

func (s *planStack) current() planFrame {
	assert(s != nil, "plan stack is nil")
	assert(len(s.frames) > 0, "plan stack is empty")
	return s.frames[len(s.frames)-1]
}

func (s *planStack) currentFeatures() featureSet { return s.current().features }

func (s *planStack) currentPlanID() uint16 {
	f := s.current()
	return f.id
}

func (s *planStack) push(settings []FeatureSetting) (uint16, error) {
	assert(s != nil, "plan stack is nil")
	if len(s.frames) == 0 {
		return 0, errPlanStackUnderflow
	}
	nextFeatures := s.current().features.applyPush(settings)
	id := s.nextID
	if id == 0 {
		return 0, errShaper("plan id overflow")
	}
	s.nextID++
	s.frames = append(s.frames, planFrame{
		id:       id,
		features: nextFeatures,
	})
	return id, nil
}

func (s *planStack) pop() error {
	assert(s != nil, "plan stack is nil")
	if len(s.frames) <= 1 {
		return errPlanStackUnderflow
	}
	s.frames = s.frames[:len(s.frames)-1]
	return nil
}

func (s *planStack) ensureClosed() error {
	assert(s != nil, "plan stack is nil")
	if len(s.frames) != 1 {
		return errPlanStackUnclosed
	}
	return nil
}
