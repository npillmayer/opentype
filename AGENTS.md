# AGENTS.md

This file provides guidance for coding agents working in this repository. It adapts the repository guidance from CLAUDE.md into a tool-agnostic format.

## Project Overview

This is a Go package for parsing and accessing OpenType font tables and features.

- Root package (`opentype`) provides high-level font metrics types and interfaces.
- Package `ot` provides low-level access to OpenType font structures for shapers, rasterizers, and other tooling.
- `otlayout` provides higher-level API for OpenType layout feature discovery and application.
- `otquery` provides convenience queries for font information and metrics.
- `otcli` is an interactive REPL for inspecting OpenType fonts.

Key design: a lazy, error-tolerant navigation pattern ("Schrödinger's cat") that allows on-demand traversal of font data without full upfront parsing.

## Architecture

- `opentype.go`: root types for metrics and bounds.
- `ot/`: core OpenType parsing and table access.
- `otlayout/`: layout feature discovery and application.
- `otquery/`: high-level queries for names, metrics, and script support.
- `otcli/`: REPL for exploring font tables.

Memory strategy: keep original font bytes in memory, parse on demand, avoid extensive copying.

### Core files

- `ot/ot.go`: core Font type, table access interface, Table abstraction.
- `ot/parse.go`: parsing entry point, shared layout utilities, validation.
- `ot/parse_gsub.go`: GSUB parsing.
- `ot/parse_gpos.go`: GPOS parsing.
- `ot/bytes.go`: binary navigation primitives (binarySegm).
- `ot/factory.go`: navigation pattern (NavLink, NavList, NavMap).
- `ot/errors.go`: error collection (FontError, FontWarning, severities).
- `ot/option.go`: option monad for optional values.

### Query package (`otquery/`)

Key functions:

- `FontType(font)`
- `NameInfo(font, lang)`
- `LayoutTables(font)`
- `FontSupportsScript(font, script, lang)` (with DFLT fallback)
- `ClassesForGlyph(font, gid)`
- `FontMetrics(font)` (OS/2 fallback when hhea values are zero)
- `GlyphIndex(font, codepoint)`
- `CodePointForGlyph(font, gid)` (sequential search)
- `GlyphMetrics(font, gid)`

`kern.go` is a placeholder for future kerning support.

### CLI tool (`otcli/`)

Single-file REPL in `otcli/main.go` using:

- `github.com/chzyer/readline` for input
- `github.com/pterm/pterm` for display
- a stack-based navigation model over `ot.Table` and `ot.Navigator`

Key commands:

- `table:<tag>`
- `map[:<key>]`
- `list[:<index>]`
- `scripts[:<tag>]`
- `features[:<index>]`
- `->` to follow links
- `help[:<topic>]`
- `quit`

## Navigation Pattern: "Schrödinger's Cat"

Navigation is lazy and error-tolerant. If a step fails, downstream calls return empty/void results. Errors are carried and can be retrieved via `Error()`.

Abstractions:

1. **NavLink**: pointer to another location (`Jump`, `Navigate`, `IsNull`).
2. **NavList**: list/array access (`Len`, `Get`, `All`).
3. **NavMap**: lookup table (`Lookup`, `LookupTag`, TagRecordMap variant).

## Parsing Strategy

Entry point: `Parse(font []byte) -> *Font` (see `ot/parse.go`).

Flow:

1. Read header and table directory.
2. Parse individual tables via dispatcher.
3. Extract layout info (GSUB/GPOS/GDEF/BASE).
4. Validate cross-table consistency.

Parsing split:

- `parse.go`: core parsing and shared layout utilities.
- `parse_gsub.go`: GSUB-specific parsing.
- `parse_gpos.go`: GPOS-specific parsing.

## Supported Tables

- Required semantic tables: `cmap`, `head`, `hhea`, `hmtx`, `maxp`, `loca`, `kern`.
- Layout tables: `GSUB`, `GPOS`, `GDEF`, `BASE`.
- Generic access via `font.Table(tag)` and `.Fields()`.

GSUB: all 8 lookup types implemented.
GPOS: all 9 lookup types implemented.

## Error Handling

Dual strategy:

- Errors and warnings collected during `Parse()`.
- Non-critical errors do not abort parse; use `font.Errors()`, `font.Warnings()`, `font.CriticalErrors()`.

Severity levels: Critical, Major, Minor; warnings are separate. Many real fonts include minor issues.

Remaining TODO: optional collection for lazy-navigation errors (see `TODO_ERROR_COLLECTION.md`).

## Safety Features

- Arithmetic safety (`checkedMul*`, `checkedAdd*`).
- Bounds validation for slices/offsets.
- Depth limits for extensions and indirections.
- Cross-table consistency checks (hhea/hmtx, head/loca, cmap glyph bounds).

## Development Commands

- Build: `go build ./...`
- Tests: `go test -v ./...` or `go test -v ./ot`
- Race: `go test -race ./...`
- Format: `go fmt ./...`
- Vet: `go vet ./...`
- Tidy: `go mod tidy`

## Current Limitations

- No variable fonts.
- No font collections (.ttc).
- No color emoji tables (COLR/CPAL/CBDT/CBLC/sbix).
- No vertical metrics (VORG/vmtx/vhea).

## Design Principles

- Memory-safe, bounds-checked.
- Lazy parsing, zero-copy where possible.
- Error-tolerant navigation.
- OpenType 1.9 compatible.

## Dependencies

- `golang.org/x/image`
- `golang.org/x/text`
- `github.com/npillmayer/schuko`
- `github.com/npillmayer/tyse`

## Tests

- `ot/nav_test.go`: navigation pattern tests.
- `ot/parse_test.go`: parsing validation.
- `ot/ot_test.go`: core functionality.

## Code Patterns

Reading a table:

```go
cmap := font.CMap
os2 := font.Table(T("OS/2"))
xAvgCharWidth := os2.Fields().Get(1).U16(0)
```

Navigation chain:

```go
features := font.Layout.GSub.ScriptList
    .LookupTag(scriptTag)
    .Navigate()
    .Map()
    .LookupTag(langTag)
    .Navigate()
    .List()

if features.IsVoid() {
    err := features.Error()
    _ = err
}
```

Safe binary reading:

```go
seg := binarySegm(rawBytes)
version := seg.U16(0)
count := seg.U32(2)
subseg, ok := seg.view(6, 10)
_ = subseg
_ = ok
```
