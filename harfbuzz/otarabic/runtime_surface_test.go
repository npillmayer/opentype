package otarabic

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/harfbuzz"
)

type runtimeBufferSurface interface {
	MergeClusters(start, end int)
	UnsafeToBreak(start, end int)
	UnsafeToConcat(start, end int)
	UnsafeToConcatFromOutbuffer(start, end int)
	SafeToInsertTatweel(start, end int)
	PreContext() []rune
	PostContext() []rune
}

type runtimeGlyphSurface interface {
	Codepoint() rune
	SetCodepoint(rune)
	ComplexAux() uint8
	SetComplexAux(uint8)
	ModifiedCombiningClass() uint8
	SetModifiedCombiningClass(uint8)
	GeneralCategory() uint8
	IsDefaultIgnorable() bool
	Multiplied() bool
	LigComp() uint8
}

var (
	_ runtimeBufferSurface = (*harfbuzz.Buffer)(nil)
	_ runtimeGlyphSurface  = (*harfbuzz.GlyphInfo)(nil)
)

func TestRuntimeSurfaceCompiles(t *testing.T) {}

func TestShaperHookSurface(t *testing.T) {
	t.Helper()

	engine := New()

	if _, ok := engine.(harfbuzz.ShapingEnginePolicy); !ok {
		t.Fatal("arabic shaper must implement policy hooks")
	}
	if _, ok := engine.(harfbuzz.ShapingEnginePlanHooks); !ok {
		t.Fatal("arabic shaper must implement plan hooks")
	}
	if _, ok := engine.(harfbuzz.ShapingEnginePostResolveHook); !ok {
		t.Fatal("arabic shaper must implement post-resolve hooks")
	}
	if _, ok := engine.(harfbuzz.ShapingEnginePreGSUBHook); !ok {
		t.Fatal("arabic shaper must implement pre-GSUB hooks")
	}
	if _, ok := engine.(harfbuzz.ShapingEngineMaskHook); !ok {
		t.Fatal("arabic shaper must implement mask hooks")
	}
	if _, ok := engine.(harfbuzz.ShapingEngineReorderHook); !ok {
		t.Fatal("arabic shaper must implement reorder hooks")
	}
	if _, ok := engine.(harfbuzz.ShapingEnginePostprocessHook); !ok {
		t.Fatal("arabic shaper must implement postprocess hooks")
	}

	// Keep the Arabic hook surface narrow: no preprocess/compose/decompose custom hooks.
	if _, ok := engine.(harfbuzz.ShapingEnginePreprocessHook); ok {
		t.Fatal("arabic shaper must not implement preprocess hook")
	}
	if _, ok := engine.(harfbuzz.ShapingEngineComposeHook); ok {
		t.Fatal("arabic shaper must not implement compose hook")
	}
	if _, ok := engine.(harfbuzz.ShapingEngineDecomposeHook); ok {
		t.Fatal("arabic shaper must not implement decompose hook")
	}
}

func TestNoBaseArabicBridgeSelectors(t *testing.T) {
	t.Helper()

	const basePkgPath = "github.com/npillmayer/opentype/harfbuzz"
	forbidden := map[string]struct{}{
		"ArabicJoiningType":        {},
		"ArabicIsWord":             {},
		"NewArabicFallbackPlan":    {},
		"NewArabicFallbackProgram": {},
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		baseImportNames := map[string]struct{}{}
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, "\"")
			if path != basePkgPath {
				continue
			}
			if imp.Name != nil && imp.Name.Name != "." && imp.Name.Name != "_" {
				baseImportNames[imp.Name.Name] = struct{}{}
				continue
			}
			baseImportNames["harfbuzz"] = struct{}{}
		}
		if len(baseImportNames) == 0 {
			continue
		}

		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if _, ok := baseImportNames[ident.Name]; !ok {
				return true
			}
			if _, bad := forbidden[sel.Sel.Name]; bad || strings.HasPrefix(sel.Sel.Name, "Arabic") {
				t.Errorf("%s references forbidden base selector %s.%s", name, ident.Name, sel.Sel.Name)
			}
			return true
		})
	}
}
