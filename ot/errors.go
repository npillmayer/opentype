package ot

import "fmt"

// ErrorSeverity represents the severity level of a font parsing error.
type ErrorSeverity int

const (
	// SeverityCritical indicates a severe error that makes the font unusable or unreliable.
	SeverityCritical ErrorSeverity = iota
	// SeverityMajor indicates a significant error that may affect functionality but doesn't prevent usage.
	SeverityMajor
	// SeverityMinor indicates a minor issue that can be safely ignored in most cases.
	SeverityMinor
)

// String returns a human-readable representation of the error severity.
func (s ErrorSeverity) String() string {
	switch s {
	case SeverityCritical:
		return "CRITICAL"
	case SeverityMajor:
		return "MAJOR"
	case SeverityMinor:
		return "MINOR"
	default:
		return "UNKNOWN"
	}
}

// FontError represents an error encountered during font parsing.
// Errors are accumulated during initial parsing and can be inspected after parsing completes.
type FontError struct {
	Table    Tag           // The OpenType table where the error occurred (e.g., "GSUB", "GPOS")
	Section  string        // Specific section within the table (e.g., "LookupType6", "ScriptList")
	Issue    string        // Human-readable description of the issue
	Severity ErrorSeverity // Severity level of the error
	Offset   uint32        // Byte offset in the font file where the error occurred (0 if unknown)
}

// Error implements the error interface.
func (e FontError) Error() string {
	if e.Offset > 0 {
		return fmt.Sprintf("[%s] %s/%s at offset %d: %s", e.Severity, e.Table, e.Section, e.Offset, e.Issue)
	}
	return fmt.Sprintf("[%s] %s/%s: %s", e.Severity, e.Table, e.Section, e.Issue)
}

// FontWarning represents a non-critical issue encountered during font parsing.
// Warnings indicate potential problems but do not prevent font usage.
type FontWarning struct {
	Table  Tag    // The OpenType table where the warning occurred
	Issue  string // Human-readable description of the warning
	Offset uint32 // Byte offset in the font file where the warning occurred (0 if unknown)
}

// String returns a human-readable representation of the warning.
func (w FontWarning) String() string {
	if w.Offset > 0 {
		return fmt.Sprintf("[WARNING] %s at offset %d: %s", w.Table, w.Offset, w.Issue)
	}
	return fmt.Sprintf("[WARNING] %s: %s", w.Table, w.Issue)
}

// errorCollector accumulates errors and warnings during font parsing.
// This is an internal helper used by the parser to collect issues as they are discovered.
type errorCollector struct {
	errors   []FontError
	warnings []FontWarning
}

// addError records a parsing error.
func (ec *errorCollector) addError(table Tag, section string, issue string, severity ErrorSeverity, offset uint32) {
	ec.errors = append(ec.errors, FontError{
		Table:    table,
		Section:  section,
		Issue:    issue,
		Severity: severity,
		Offset:   offset,
	})
}

// addWarning records a parsing warning.
func (ec *errorCollector) addWarning(table Tag, issue string, offset uint32) {
	ec.warnings = append(ec.warnings, FontWarning{
		Table:  table,
		Issue:  issue,
		Offset: offset,
	})
}

// hasErrors returns true if any errors have been recorded.
func (ec *errorCollector) hasErrors() bool {
	return len(ec.errors) > 0
}

// hasWarnings returns true if any warnings have been recorded.
func (ec *errorCollector) hasWarnings() bool {
	return len(ec.warnings) > 0
}

// criticalErrors returns all errors with critical severity.
func (ec *errorCollector) criticalErrors() []FontError {
	critical := make([]FontError, 0)
	for _, err := range ec.errors {
		if err.Severity == SeverityCritical {
			critical = append(critical, err)
		}
	}
	return critical
}

// hasCriticalErrors returns true if any critical errors have been recorded.
func (ec *errorCollector) hasCriticalErrors() bool {
	for _, err := range ec.errors {
		if err.Severity == SeverityCritical {
			return true
		}
	}
	return false
}
