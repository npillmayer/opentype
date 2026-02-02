# TODO: Error Collection for Lazy/Navigation Parsing

## Overview

The main upfront parsing paths now have dual recording (tracer logs + error collection) implemented. However, lazy parsing functions that are called during navigation still use `errFontFormat()` without error collection. These functions need to be refactored to support error collection.

## Functions Requiring Error Collection

### 1. parseClassDefinitions() - ot/parse.go:1217

**Current signature:**
```go
func parseClassDefinitions(b binarySegm) (ClassDefinitions, error)
```

**Needs to become:**
```go
func parseClassDefinitions(b binarySegm, tag Tag, offset uint32, ec *errorCollector) (ClassDefinitions, error)
```

**Error points to convert:**
- Line ~1223: `errFontFormat("ClassDef table too small")`
- Line ~1236: `errFontFormat("ClassDef format 1 header incomplete")`
- Line ~1249: `errFontFormat("ClassDef format 2 header incomplete")`
- Line ~1258: `errFontFormat(fmt.Sprintf("unknown ClassDef format %d", cdef.format))`

**Callers to update:**
- `parseGlyphClassDefinitions()` in ot/parse.go
- `parseMarkAttachmentClassDef()` in ot/parse.go
- `parseSequenceContextFormat2()` in ot/parse.go
- `parseChainedSequenceContextFormat2()` in ot/parse.go
- Any other places where ClassDef tables are parsed during navigation

**Challenge:** These are called during lazy navigation, which currently doesn't have access to error collector. Will need to:
1. Thread `ec` through the navigation API, OR
2. Create a separate error collection mechanism for navigation errors, OR
3. Store navigation errors in the Font struct for retrieval after navigation

### 2. parseSequenceContext*() functions - ot/parse.go:1310-1400

**Functions:**
- `parseSequenceContext()`
- `parseSequenceContextFormat1()`
- `parseSequenceContextFormat2()`
- `parseSequenceContextFormat3()`

**Current signatures:**
```go
func parseSequenceContext(b binarySegm, sub LookupSubtable) (LookupSubtable, error)
func parseSequenceContextFormat1(b binarySegm, sub LookupSubtable) (LookupSubtable, error)
func parseSequenceContextFormat2(b binarySegm, sub LookupSubtable) (LookupSubtable, error)
func parseSequenceContextFormat3(b binarySegm, sub LookupSubtable) (LookupSubtable, error)
```

**Needs to become:**
```go
func parseSequenceContext(b binarySegm, sub LookupSubtable, tag Tag, offset uint32, ec *errorCollector) (LookupSubtable, error)
// ... etc for other functions
```

**Error points to convert:**
- Line ~1316: `errFontFormat("corrupt sequence context")` (Format detection)
- Line ~1327: `errFontFormat(fmt.Sprintf("unknown sequence context format %d", sub.Format))`
- Line ~1338: `errFontFormat("corrupt sequence context")` (Format 1)
- Line ~1363: `errFontFormat("corrupt sequence context")` (Format 2)

**Callers to update:**
- `parseGSubLookupSubtableType5()` in ot/parse_gsub.go
- `parseGPosLookupSubtableType7()` in ot/parse_gpos.go
- Any GSUB/GPOS context parsing

**Challenge:** These are called during lookup subtable parsing, which happens lazily when a specific lookup is accessed during shaping. Need to design how errors during lazy lookup parsing are reported.

### 3. parseChainedSequenceContext*() functions - ot/parse.go:1387-1425

**Functions:**
- `parseChainedSequenceContext()`
- `parseChainedSequenceContextFormat1()`
- `parseChainedSequenceContextFormat2()`
- `parseChainedSequenceContextFormat3()`

**Current signatures:**
```go
func parseChainedSequenceContext(b binarySegm, sub LookupSubtable) (LookupSubtable, error)
// ... etc
```

**Error points to convert:**
- Line ~1397: `errFontFormat("corrupt chained sequence context")`
- Line ~1409: `errFontFormat(fmt.Sprintf("unknown chained sequence context format %d", sub.Format))`
- Line ~1417: `errFontFormat("corrupt chained sequence context (format 2)")`
- Line ~1435: `errFontFormat("corrupt chained sequence context (format 3)")`

**Callers to update:**
- `parseGSubLookupSubtableType6()` in ot/parse_gsub.go
- `parseGPosLookupSubtableType8()` in ot/parse_gpos.go

**Challenge:** Same as parseSequenceContext - lazy evaluation during shaping.

### 4. parseNames() - ot/parse.go:753

**Current signature:**
```go
func parseNames(b binarySegm) (nameNames, error)
```

**Needs to become:**
```go
func parseNames(b binarySegm, tag Tag, offset uint32, ec *errorCollector) (nameNames, error)
```

**Error points to convert:**
- Line ~757: `errFontFormat("name section corrupt")`
- Line ~765: `errFontFormat(fmt.Sprintf("name table string offset %d exceeds table size %d", ...))`
- Line ~773: `errFontFormat(fmt.Sprintf("name table records size overflow: %v", err))`
- Line ~777: `errFontFormat(fmt.Sprintf("name table size calculation overflow: %v", err))`
- Line ~780: `errFontFormat("name section corrupt")`

**Callers to update:**
- `ot/factory.go:104` - `names, err := parseNames(loc.Bytes())`

**Challenge:** This is called from factory.go, not during main Parse(). May need to pass Font reference to collect errors into, or return errors separately for the caller to handle.

## Implementation Strategy

### Option A: Extend Navigation API (Recommended for parseClassDefinitions)

Since ClassDef parsing happens during navigation and the results affect layout processing, consider:

1. Add error collection to navigation context
2. Store accumulated navigation errors in Font struct
3. Provide `Font.NavigationErrors()` method
4. Update all ClassDef parsing call sites

### Option B: Lazy Error Collection (Recommended for Sequence Context)

Since sequence context parsing happens during lookup subtable access:

1. Create `LookupSubtableError` type stored with the subtable
2. When parsing fails, store error in the subtable structure
3. Shaper can check for errors before using the subtable
4. Errors can be collected via `Font.LookupErrors()` after shaping

### Option C: Synchronous Upfront Parsing (Major refactor)

Parse all lookup subtables during initial Parse() instead of lazily:

1. **Pros:** All errors collected upfront, simpler error handling
2. **Cons:** Higher memory usage, slower initial parse, breaks lazy design

**Not recommended** - goes against the "Schrödinger's cat" philosophy.

### Option D: Hybrid Approach

1. **parseClassDefinitions:** Use Option A - extend navigation API
2. **parseSequenceContext*:** Use Option B - store errors with subtables
3. **parseNames:** Pass Font reference or return separate error collection

## Required Function Signature Changes

### parseClassDefinitions chain:
```go
parseClassDefinitions(b binarySegm, tag Tag, offset uint32, ec *errorCollector)
parseGlyphClassDefinitions(gdef *GDefTable, b binarySegm, err error, tag Tag, offset uint32, ec *errorCollector)
parseMarkAttachmentClassDef(gdef *GDefTable, b binarySegm, err error, tag Tag, offset uint32, ec *errorCollector)
parseContextClassDef(b binarySegm, offset int, tag Tag, tableOffset uint32, ec *errorCollector)
```

### parseSequenceContext chain:
```go
parseSequenceContext(b binarySegm, sub LookupSubtable, tag Tag, offset uint32, ec *errorCollector)
parseSequenceContextFormat1(b binarySegm, sub LookupSubtable, tag Tag, offset uint32, ec *errorCollector)
parseSequenceContextFormat2(b binarySegm, sub LookupSubtable, tag Tag, offset uint32, ec *errorCollector)
parseSequenceContextFormat3(b binarySegm, sub LookupSubtable, tag Tag, offset uint32, ec *errorCollector)
```

### Lookup subtable parsers in parse_gsub.go and parse_gpos.go:
```go
parseGSubLookupSubtableType5(..., ec *errorCollector)
parseGSubLookupSubtableType6(..., ec *errorCollector)
parseGPosLookupSubtableType7(..., ec *errorCollector)
parseGPosLookupSubtableType8(..., ec *errorCollector)
```

## Testing Strategy

1. **Unit tests** for each converted function
2. **Integration tests** for navigation with errors
3. **Regression tests** ensuring all existing tests still pass
4. **Error collection tests** verifying errors are captured correctly
5. **Font tests** with intentionally malformed ClassDef/Context tables

## Estimated Impact

- **Lines of code:** ~50-100 function signatures to update
- **Files affected:** parse.go, parse_gsub.go, parse_gpos.go, layout.go, factory.go
- **Risk level:** Medium (affects navigation API and lazy parsing)
- **Backward compatibility:** Should maintain (internal refactor only)

## Notes

- The remaining unconverted errFontFormat calls are in parse.go lines: 1223, 1236, 1249, 1258, 1316, 1327, 1338, 1363, 1397, 1409, 1417, 1435, 757, 765, 773, 777, 780
- Total estimated: ~20-25 additional error collection points
- These are less critical than upfront validation but still valuable for font debugging

## Related Documentation

- See `ERROR_HANDLING.md` for overall error handling design
- See `errors.go` for error collection types and severity guidelines
- See `ot/parse.go:91-95` for errFontFormat() helper function
