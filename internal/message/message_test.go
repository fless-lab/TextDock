package message

import (
	"errors"
	"strings"
	"testing"
)

func TestSMSAnalysisBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, text, encoding string
		units, segments      int
	}{
		{"single GSM", strings.Repeat("a", 160), "GSM-7", 160, 1},
		{"concatenated GSM", strings.Repeat("a", 161), "GSM-7", 161, 2},
		{"GSM extensions", "€{}[]^~\\|", "GSM-7", 18, 1},
		{"extension boundary", strings.Repeat("^", 153), "GSM-7", 306, 3},
		{"unicode single", strings.Repeat("漢", 70), "UTF-16", 70, 1},
		{"unicode multi", strings.Repeat("漢", 71), "UTF-16", 71, 2},
		{"surrogate pairs", strings.Repeat("🙂", 67), "UTF-16", 134, 3},
		{"accent in GSM", "été", "GSM-7", 3, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := Analyze(tc.text)
			if a.Encoding != tc.encoding || a.Units != tc.units || a.Segments != tc.segments {
				t.Fatalf("unexpected analysis: %+v", a)
			}
		})
	}
}

func TestOTPCandidates(t *testing.T) {
	for text, want := range map[string]string{
		"Your code is 482193.": "482193", "1234": "1234", "reference abc123456def": "",
		"call +33612345678": "", "123456789": "", "No code here": "",
	} {
		if got := Analyze(text).OTP; got != want {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}
}

func TestInputValidation(t *testing.T) {
	valid := Input{To: "+33612345678", From: "Acme", Body: "code 482193"}
	m, err := New(valid, "api")
	if err != nil || m.Status != "captured" || m.ID == "" || m.CreatedAt.IsZero() {
		t.Fatalf("capture: %+v %v", m, err)
	}
	for _, in := range []Input{
		{To: "0612345678", From: "Acme", Body: "hello"},
		{To: valid.To, Body: "hello"},
		{To: valid.To, From: "Acme", Body: " "},
		{To: valid.To, From: "Acme", Body: strings.Repeat("a", 4097)},
	} {
		if _, err := New(in, "api"); !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected validation error for %+v", in)
		}
	}
}
