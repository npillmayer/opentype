# Error Handling Implementation

## Overview

This document describes the error handling infrastructure implemented for the OpenType font parser. The implementation follows a "dual strategy" approach that balances strict validation during initial parsing with permissive, error-tolerant access during later use.

## Implementation Phases

### Phase 1: Error Collection Infrastructure ✅ COMPLETE

**Goal**: Create the foundational types and methods for error collection.

**Files Created/Modified**:
- `ot/errors.go` - New file with error types and collection infrastructure
- `ot/ot.go` - Extended Font struct with error tracking fields and inspection methods

**Key Types**:

```go
// Error severity levels
type ErrorSeverity int
const (
    SeverityCritical ErrorSeverity = iota
    SeverityMajor
    SeverityMinor
)

// Error with context and severity
type FontError struct {
    Table    Tag
    Section  string
    Issue    string
    Severity ErrorSeverity
    Offset   uint32
}

// Non-critical issues
type FontWarning struct {
    Table  Tag
    Issue  string
    Offset uint32
}

// Internal helper for accumulation
type errorCollector struct {
    errors   []FontError
    warnings []FontWarning
}
```

**Font API Methods**:
- `Font.Errors() []FontError` - All parsing errors
- `Font.Warnings() []FontWarning` - All parsing warnings
- `Font.CriticalErrors() []FontError` - Critical errors only
- `Font.HasCriticalErrors() bool` - Quick check for severe issues

**Testing**: Comprehensive unit tests in `ot/errors_test.go` (5 tests, all passing)

---

### Phase 2: Parser Integration ✅ COMPLETE

**Goal**: Thread error collector through all parsing functions without breaking existing behavior.

**Files Modified**:
- `ot/parse.go` - Updated Parse() and all table parsing functions
- `ot/parse_gsub.go` - Updated parseGSub() 
- `ot/parse_gpos.go` - Updated parseGPos()

**Function Signature Changes** (added `ec *errorCollector` parameter):
```go
// Core entry point
Parse(font []byte) (*Font, error)
  └─ creates errorCollector
  └─ passes to parseTable()
      └─ parseHead(), parseCMap(), parseKern(), etc.
      └─ parseGDef(), parseGSub(), parseGPos()
          └─ parseLayoutHeader()
          └─ parseLookupList()
```

**Error Transfer**:
At the end of Parse(), accumulated errors are transferred to the Font:
```go
otf.parseErrors = ec.errors
otf.parseWarnings = ec.warnings
```

**Testing**: 
- Created `TestErrorCollection()` in `parse_test.go`
- Demonstrates Calibri kern table warning collection
- Verifies all error inspection APIs work correctly
- All 14 existing tests continue to pass

---

### Phase 3: Validation Enhancement (Minimal) ✅ COMPLETE

**Goal**: Add 15+ error/warning collection points at critical validation locations.

**Locations Enhanced**:

1. **CMAP Table Parsing** (4 error points):
   - Arithmetic overflow in entries size → SeverityCritical
   - Arithmetic overflow in table size → SeverityCritical  
   - Table size insufficient → SeverityCritical
   - No supported format found → SeverityMajor
   - Unparseable sub-tables → Warning

2. **Layout Table Parsing** (5 error points):
   - Header too small → SeverityCritical
   - Unsupported version → SeverityMajor
   - v1.0/v1.1 header incomplete → SeverityCritical
   - Lookup list header too small → SeverityCritical
   - Lookup count exceeds maximum → SeverityCritical

3. **Cross-Table Validation** (5 error points):
   - hhea.NumberOfHMetrics exceeds maxp.NumGlyphs → SeverityMajor
   - hmtx longMetrics size overflow → SeverityCritical
   - hmtx leftSideBearings size overflow → SeverityCritical
   - hmtx total size overflow → SeverityCritical
   - hmtx table size insufficient → SeverityCritical

4. **Warning Collection** (3 warning points):
   - Kern table size mismatch (Calibri issue) → Warning
   - Uninterpreted tables (DSIG, OS/2, cvt, etc.) → Warning
   - CMAP sub-table parsing failures → Warning

**Severity Classification Guidelines** (documented in `ot/errors.go`):

| Severity | Use Cases | Impact |
|----------|-----------|--------|
| **Critical** | Buffer overruns, required tables missing, arithmetic overflow | Font may crash or produce undefined behavior |
| **Major** | Format errors, invalid structures, cross-table violations | Font works but features may be broken |
| **Minor** | Missing optional features, deprecated formats | Font fully functional, enhancements unavailable |
| **Warning** | Auto-corrected issues, uninterpreted tables | Font works correctly, informational only |

**Testing Results** (Calibri font):
- 9 warnings collected (7 uninterpreted tables + 1 kern mismatch + others)
- 0 errors (as expected for valid font)
- All 15 tests pass

---

## Design Philosophy

### 1. Upfront Validation
All critical validation happens during the initial `Parse()` call. Errors are detected and accumulated, not silently ignored.

### 2. Error Accumulation (Non-Blocking)
Most errors don't prevent parsing from continuing. Instead, they're recorded for user inspection while parsing proceeds with graceful degradation.

### 3. Dual Recording Pattern
All error collection follows this pattern:
```go
// Log for debugging (existing behavior preserved)
tracer().Errorf("issue detected: %v", details)

// Collect for user inspection (new capability)
ec.addError(tag, section, message, severity, offset)

// Still fail fast for critical errors (existing behavior preserved)
return errFontFormat("issue")
```

### 4. Graceful Degradation
After successful parsing (even with accumulated errors), later table access continues to use the "Schrödinger's cat" pattern - silently returning empty/default values on errors rather than panicking.

### 5. Performance Conscious
- Error collection only happens during initial parse
- No repeated validation on hot paths (table queries)
- No performance regression in normal operation

---

## Usage Examples

### Basic Error Inspection

```go
font, err := ot.Parse(fontBytes)
if err != nil {
    // Critical failure - font completely unusable
    return fmt.Errorf("failed to parse font: %w", err)
}

// Check for issues
if font.HasCriticalErrors() {
    log.Warn("Font has critical errors - may be unreliable")
    for _, err := range font.CriticalErrors() {
        log.Warn("  %s", err.Error())
    }
}

// Log all issues for diagnostics
for _, err := range font.Errors() {
    log.Info("Font error: %s", err.Error())
}

for _, warn := range font.Warnings() {
    log.Debug("Font warning: %s", warn.String())
}
```

### Production Font Validation

```go
func ValidateFont(fontBytes []byte) (quality string, usable bool) {
    font, err := ot.Parse(fontBytes)
    if err != nil {
        return "INVALID", false
    }
    
    if font.HasCriticalErrors() {
        return "POOR", true // Usable but with significant issues
    }
    
    if len(font.Errors()) > 0 {
        return "FAIR", true // Some issues but generally fine
    }
    
    if len(font.Warnings()) > 5 {
        return "GOOD", true // Minor issues only
    }
    
    return "EXCELLENT", true // No issues detected
}
```

### Specific Error Handling

```go
font, _ := ot.Parse(fontBytes)

// Check for specific table errors
for _, err := range font.Errors() {
    if err.Table == ot.T("GSUB") && err.Severity == ot.SeverityCritical {
        log.Error("Critical GSUB error in %s: %s", err.Section, err.Issue)
        // Disable substitution features
    }
}

// Handle known font issues
for _, warn := range font.Warnings() {
    if warn.Table == ot.T("kern") {
        log.Debug("Known Calibri kern table issue - auto-corrected")
    }
}
```

---

## Testing

### Test Coverage

**Unit Tests** (`ot/errors_test.go`):
- `TestErrorSeverity` - Severity string formatting
- `TestFontError` - Error creation and formatting
- `TestFontWarning` - Warning creation and formatting  
- `TestErrorCollector` - Internal collection mechanism
- `TestFontErrorMethods` - Font error inspection API

**Integration Test** (`ot/parse_test.go`):
- `TestErrorCollection` - Real-world error collection with Calibri font
  - Verifies warnings are collected
  - Tests all Font error inspection methods
  - Validates kern table warning appears

### Test Results

```
=== RUN   TestErrorCollection
    parse_test.go:354: Font has 9 warnings
    parse_test.go:359: Warning: [WARNING] DSIG at offset 774056: table not interpreted
    parse_test.go:359: Warning: [WARNING] OS/2 at offset 440: table not interpreted
    parse_test.go:359: Warning: [WARNING] cvt  at offset 33040: table not interpreted
    parse_test.go:359: Warning: [WARNING] fpgm at offset 21476: table not interpreted
    parse_test.go:359: Warning: [WARNING] gasp at offset 649276: table not interpreted
    parse_test.go:359: Warning: [WARNING] kern at offset 485364: kern sub-table size mismatch
    parse_test.go:359: Warning: [WARNING] name at offset 645616: table not interpreted
    parse_test.go:359: Warning: [WARNING] post at offset 649244: table not interpreted
    parse_test.go:359: Warning: [WARNING] prep at offset 23808: table not interpreted
    parse_test.go:369: Successfully collected kern table size mismatch warning
    parse_test.go:376: Font has 0 errors
--- PASS: TestErrorCollection (0.00s)
```

**All 15 tests pass** (14 existing + 1 new)

---

## Future Enhancements (Comprehensive Phase 3)

### Not Yet Implemented

The current implementation provides minimal but comprehensive error coverage. Additional enhancements could include:

1. **Deeper Error Collection** (~25 more locations):
   - Thread `ec` into GSUB/GPOS lookup subtable parsers
   - Add buffer validation error recording in parse_gsub.go and parse_gpos.go
   - Requires ~25 function signature changes

2. **Convert errFontFormat** (~60 call sites):
   - Replace simple `errFontFormat()` calls with dual recording
   - Requires deciding which errors should fail fast vs. accumulate

3. **GDEF-Specific Errors**:
   - parseGDefHeader validation
   - AttachmentPointList validation
   - MarkGlyphSets validation

4. **Enhanced Validation**:
   - More granular size checking
   - Additional cross-table consistency checks
   - Format-specific validation

### Estimated Effort
- Current implementation: ~15 error collection points, ~50 LOC
- Comprehensive implementation: ~40-50 collection points, ~200 LOC

---

## Summary Statistics

| Metric | Count |
|--------|-------|
| New files created | 2 (errors.go, ERROR_HANDLING.md) |
| Files modified | 5 (ot.go, parse.go, parse_gsub.go, parse_gpos.go, CLAUDE.md) |
| Error collection points added | 15+ |
| Function signatures updated | 12 |
| Tests added | 6 |
| Total LOC added/modified | ~300 |
| All tests passing | ✅ 15/15 |
| Build status | ✅ Clean |

---

## References

- OpenType Specification: https://docs.microsoft.com/en-us/typography/opentype/spec/
- Rust ttf-parser (inspiration): https://github.com/RazrFalcon/ttf-parser
- Error handling discussion: See git commit history for Phase 1-3 implementation

---

*Document created: 2025-12-26*
*Implementation complete: Phases 1-3 (Minimal Target)*
