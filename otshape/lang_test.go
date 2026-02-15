package otshape

import (
	"path/filepath"
	"testing"

	"github.com/npillmayer/opentype"
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/schuko/tracing"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
	"github.com/stretchr/testify/suite"
	"golang.org/x/text/language"
)

// --- Test Suite Preparation ------------------------------------------------

type LanguageTestEnviron struct {
	suite.Suite
	calibri *ot.Font
}

// listen for 'go test' command --> run test methods
func TestLanguageFunctions(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "tyse.fonts")
	defer teardown()
	suite.Run(t, new(LanguageTestEnviron))
}

// run once, before test suite methods
func (env *LanguageTestEnviron) SetupSuite() {
	env.T().Log("Setting up test suite")
	tracing.Select("tyse.fonts").SetTraceLevel(tracing.LevelError)
	env.calibri = loadLocalFont(env.T(), "Calibri.ttf")
	tracing.Select("tyse.fonts").SetTraceLevel(tracing.LevelInfo)
}

// run once, after test suite methods
func (env *LanguageTestEnviron) TearDownSuite() {
	env.T().Log("Tearing down test suite")
}

// --- Tests -----------------------------------------------------------------

func (env *LanguageTestEnviron) TestLanguageTagForLanguage() {
	langs := []struct {
		in  string
		out string
	}{
		{"DE", "DEU"},
		{"DE_de", "DEU"},
		{"DE_ch", "DEU"},
		{"EN_us", "ENG"},
	}
	for _, pair := range langs {
		tag := LanguageTagForLanguage(language.Make(pair.in), language.High)
		env.Equal(ot.T(pair.out).String(), tag.String(), "expected language match %s", pair.out)
	}
}

// --- Helpers ---------------------------------------------------------------

func loadLocalFont(t *testing.T, fontFileName string) *ot.Font {
	path := filepath.Join("..", "testdata", fontFileName)
	f, err := opentype.LoadOpenTypeFont(path)
	if err != nil {
		t.Fatalf("cannot load test font %s: %s", fontFileName, err)
	}
	t.Logf("loaded SFNT font = %s", f.Fontname)
	otf, err := ot.Parse(f.Binary)
	if err != nil {
		t.Fatalf("cannot decode test font %s: %s", fontFileName, err)
	}
	t.Logf("parsed OpenType font from %s", f.Fontname)
	return otf
}
