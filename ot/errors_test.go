package ot

import "testing"

// TestErrorSeverity verifies the ErrorSeverity String() method.
func TestErrorSeverity(t *testing.T) {
	tests := []struct {
		severity ErrorSeverity
		expected string
	}{
		{SeverityCritical, "CRITICAL"},
		{SeverityMajor, "MAJOR"},
		{SeverityMinor, "MINOR"},
		{ErrorSeverity(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		result := tt.severity.String()
		if result != tt.expected {
			t.Errorf("ErrorSeverity(%d).String() = %q; want %q", tt.severity, result, tt.expected)
		}
	}
}

// TestFontError verifies FontError creation and formatting.
func TestFontError(t *testing.T) {
	tests := []struct {
		name     string
		err      FontError
		expected string
	}{
		{
			name: "Error with offset",
			err: FontError{
				Table:    T("GSUB"),
				Section:  "LookupType6",
				Issue:    "Buffer too small",
				Severity: SeverityCritical,
				Offset:   1234,
			},
			expected: "[CRITICAL] GSUB/LookupType6 at offset 1234: Buffer too small",
		},
		{
			name: "Error without offset",
			err: FontError{
				Table:    T("GPOS"),
				Section:  "LookupType2",
				Issue:    "Invalid format",
				Severity: SeverityMajor,
				Offset:   0,
			},
			expected: "[MAJOR] GPOS/LookupType2: Invalid format",
		},
		{
			name: "Minor error",
			err: FontError{
				Table:    T("GDEF"),
				Section:  "GlyphClassDef",
				Issue:    "Missing coverage",
				Severity: SeverityMinor,
				Offset:   0,
			},
			expected: "[MINOR] GDEF/GlyphClassDef: Missing coverage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("FontError.Error() = %q; want %q", result, tt.expected)
			}
		})
	}
}

// TestFontWarning verifies FontWarning creation and formatting.
func TestFontWarning(t *testing.T) {
	tests := []struct {
		name     string
		warning  FontWarning
		expected string
	}{
		{
			name: "Warning with offset",
			warning: FontWarning{
				Table:  T("kern"),
				Issue:  "Table size mismatch",
				Offset: 5678,
			},
			expected: "[WARNING] kern at offset 5678: Table size mismatch",
		},
		{
			name: "Warning without offset",
			warning: FontWarning{
				Table:  T("GSUB"),
				Issue:  "Unused lookup",
				Offset: 0,
			},
			expected: "[WARNING] GSUB: Unused lookup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.warning.String()
			if result != tt.expected {
				t.Errorf("FontWarning.String() = %q; want %q", result, tt.expected)
			}
		})
	}
}

// TestErrorCollector verifies the errorCollector helper type.
func TestErrorCollector(t *testing.T) {
	ec := &errorCollector{}

	// Initially empty
	if ec.hasErrors() {
		t.Error("errorCollector should not have errors initially")
	}
	if ec.hasWarnings() {
		t.Error("errorCollector should not have warnings initially")
	}
	if ec.hasCriticalErrors() {
		t.Error("errorCollector should not have critical errors initially")
	}

	// Add a minor error
	ec.addError(T("GSUB"), "Test", "Minor issue", SeverityMinor, 100)
	if !ec.hasErrors() {
		t.Error("errorCollector should have errors after adding one")
	}
	if ec.hasCriticalErrors() {
		t.Error("errorCollector should not have critical errors yet")
	}
	if len(ec.errors) != 1 {
		t.Errorf("errorCollector should have 1 error; got %d", len(ec.errors))
	}

	// Add a critical error
	ec.addError(T("GPOS"), "Test", "Critical issue", SeverityCritical, 200)
	if !ec.hasCriticalErrors() {
		t.Error("errorCollector should have critical errors after adding one")
	}
	if len(ec.errors) != 2 {
		t.Errorf("errorCollector should have 2 errors; got %d", len(ec.errors))
	}

	// Add a major error
	ec.addError(T("GDEF"), "Test", "Major issue", SeverityMajor, 300)
	if len(ec.errors) != 3 {
		t.Errorf("errorCollector should have 3 errors; got %d", len(ec.errors))
	}

	// Check critical errors filtering
	criticalErrs := ec.criticalErrors()
	if len(criticalErrs) != 1 {
		t.Errorf("errorCollector should have 1 critical error; got %d", len(criticalErrs))
	}
	if criticalErrs[0].Severity != SeverityCritical {
		t.Error("criticalErrors() should return only critical severity errors")
	}

	// Add a warning
	ec.addWarning(T("kern"), "Warning issue", 400)
	if !ec.hasWarnings() {
		t.Error("errorCollector should have warnings after adding one")
	}
	if len(ec.warnings) != 1 {
		t.Errorf("errorCollector should have 1 warning; got %d", len(ec.warnings))
	}
}

// TestFontErrorMethods verifies Font error inspection methods.
func TestFontErrorMethods(t *testing.T) {
	// Create a Font with errors and warnings
	font := &Font{
		parseErrors: []FontError{
			{
				Table:    T("GSUB"),
				Section:  "Test1",
				Issue:    "Minor issue",
				Severity: SeverityMinor,
				Offset:   100,
			},
			{
				Table:    T("GPOS"),
				Section:  "Test2",
				Issue:    "Critical issue",
				Severity: SeverityCritical,
				Offset:   200,
			},
			{
				Table:    T("GDEF"),
				Section:  "Test3",
				Issue:    "Major issue",
				Severity: SeverityMajor,
				Offset:   300,
			},
		},
		parseWarnings: []FontWarning{
			{
				Table:  T("kern"),
				Issue:  "Warning issue",
				Offset: 400,
			},
		},
	}

	// Test Errors()
	errors := font.Errors()
	if len(errors) != 3 {
		t.Errorf("Font.Errors() should return 3 errors; got %d", len(errors))
	}

	// Test Warnings()
	warnings := font.Warnings()
	if len(warnings) != 1 {
		t.Errorf("Font.Warnings() should return 1 warning; got %d", len(warnings))
	}

	// Test CriticalErrors()
	criticalErrs := font.CriticalErrors()
	if len(criticalErrs) != 1 {
		t.Errorf("Font.CriticalErrors() should return 1 critical error; got %d", len(criticalErrs))
	}
	if criticalErrs[0].Severity != SeverityCritical {
		t.Error("Font.CriticalErrors() should return only critical severity errors")
	}

	// Test HasCriticalErrors()
	if !font.HasCriticalErrors() {
		t.Error("Font.HasCriticalErrors() should return true")
	}

	// Test with empty font
	emptyFont := &Font{}
	if len(emptyFont.Errors()) != 0 {
		t.Error("Empty font should return empty errors slice")
	}
	if len(emptyFont.Warnings()) != 0 {
		t.Error("Empty font should return empty warnings slice")
	}
	if len(emptyFont.CriticalErrors()) != 0 {
		t.Error("Empty font should return empty critical errors slice")
	}
	if emptyFont.HasCriticalErrors() {
		t.Error("Empty font should not have critical errors")
	}
}
