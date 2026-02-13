package otarabic

import "testing"

func TestResolveJoiningFormsBasic(t *testing.T) {
	// beh + beh + beh
	forms := resolveJoiningForms([]rune{'\u0628', '\u0628', '\u0628'})
	if len(forms) != 3 {
		t.Fatalf("forms length = %d, want 3", len(forms))
	}
	if forms[0] != formInit {
		t.Fatalf("first form = %d, want init(%d)", forms[0], formInit)
	}
	if forms[1] != formMedi {
		t.Fatalf("middle form = %d, want medi(%d)", forms[1], formMedi)
	}
	if forms[2] != formFina {
		t.Fatalf("last form = %d, want fina(%d)", forms[2], formFina)
	}
}

func TestResolveJoiningFormsWithRightJoiningLetter(t *testing.T) {
	// beh + alef => init + fina
	forms := resolveJoiningForms([]rune{'\u0628', '\u0627'})
	if forms[0] != formInit {
		t.Fatalf("beh form = %d, want init(%d)", forms[0], formInit)
	}
	if forms[1] != formFina {
		t.Fatalf("alef form = %d, want fina(%d)", forms[1], formFina)
	}
}

func TestResolveJoiningFormsSkipsTransparentMarks(t *testing.T) {
	// beh + fatha + beh => first joins with second base across transparent mark
	forms := resolveJoiningForms([]rune{'\u0628', '\u064E', '\u0628'})
	if forms[0] != formInit {
		t.Fatalf("first form = %d, want init(%d)", forms[0], formInit)
	}
	if forms[1] != formNone {
		t.Fatalf("mark form = %d, want none(%d)", forms[1], formNone)
	}
	if forms[2] != formFina {
		t.Fatalf("last form = %d, want fina(%d)", forms[2], formFina)
	}
}

func TestResolveJoiningFormsNonArabicAreNone(t *testing.T) {
	forms := resolveJoiningForms([]rune{'A', 'B'})
	if forms[0] != formNone || forms[1] != formNone {
		t.Fatalf("latin forms = %v, want [%d %d]", forms, formNone, formNone)
	}
}
