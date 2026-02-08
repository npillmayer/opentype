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

func TestNoBaseArabicBridgeSelectors(t *testing.T) {
	t.Helper()

	const basePkgPath = "github.com/npillmayer/opentype/harfbuzz"
	forbidden := map[string]struct{}{
		"ArabicJoiningType": {},
		"ArabicIsWord":      {},
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
			if _, bad := forbidden[sel.Sel.Name]; bad {
				t.Errorf("%s references forbidden base selector %s.%s", name, ident.Name, sel.Sel.Name)
			}
			return true
		})
	}
}
