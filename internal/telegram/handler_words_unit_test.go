package telegram

import (
	"testing"

	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/user"
	"makeMe/internal/domain/words"
)

func TestParseWordsMC(t *testing.T) {
	sid, wid, mode, chosen, correct, ok := parseWordsMC("words:mc:42:1337:enrm:2:0")
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if sid != 42 || wid != 1337 || mode != words.ModeEnRuMC || chosen != 2 || correct != 0 {
		t.Fatalf("unexpected: sid=%d wid=%d mode=%s chosen=%d correct=%d", sid, wid, mode, chosen, correct)
	}
}

func TestParseWordsMC_Invalid(t *testing.T) {
	_, _, _, _, _, ok := parseWordsMC("words:mc:42:1337:enrm:2") // too few parts
	if ok {
		t.Fatalf("expected ok=false")
	}
	_, _, _, _, _, ok = parseWordsMC("words:mc:42:1337:zzzz:0:1") // bad mode
	if ok {
		t.Fatalf("expected ok=false on unknown mode")
	}
	_, _, _, _, _, ok = parseWordsMC("words:mc:42:1337:enrm:abc:1") // bad chosen
	if ok {
		t.Fatalf("expected ok=false on non-numeric chosen")
	}
}

func TestParseWordsIdk(t *testing.T) {
	sid, wid, mode, ok := parseWordsIdk("words:idk:42:1337:rent")
	if !ok || sid != 42 || wid != 1337 || mode != words.ModeRuEnTyping {
		t.Fatalf("unexpected parse: sid=%d wid=%d mode=%s ok=%v", sid, wid, mode, ok)
	}
}

func TestParseWordsKnow(t *testing.T) {
	sid, wid, ok := parseWordsKnow("words:know:1:99")
	if !ok || sid != 1 || wid != 99 {
		t.Fatalf("unexpected parse: sid=%d wid=%d ok=%v", sid, wid, ok)
	}
}

func TestParseWordsDefer(t *testing.T) {
	sid, wid, ok := parseWordsDefer("words:defer:7:12345")
	if !ok || sid != 7 || wid != 12345 {
		t.Fatalf("unexpected parse: sid=%d wid=%d ok=%v", sid, wid, ok)
	}
	if _, _, ok := parseWordsDefer("words:defer:7"); ok {
		t.Fatalf("expected ok=false for too few parts")
	}
	if _, _, ok := parseWordsDefer("words:defer:abc:12"); ok {
		t.Fatalf("expected ok=false for non-numeric sid")
	}
}

func TestBuildMCOptions_CorrectIdxFound(t *testing.T) {
	correct := "house"
	distractors := []srs.Word{
		{English: "tree", Russian: "дерево", Level: user.B1},
		{English: "car", Russian: "машина", Level: user.B1},
		{English: "book", Russian: "книга", Level: user.B1},
	}
	options, idx := buildMCOptions(correct, distractors, englishPicker)
	if len(options) != 4 {
		t.Fatalf("expected 4 options, got %d", len(options))
	}
	if options[idx] != correct {
		t.Fatalf("correctIdx points to %q, expected %q", options[idx], correct)
	}
}

func TestBuildMCOptions_FiltersDuplicates(t *testing.T) {
	correct := "house"
	distractors := []srs.Word{
		{English: "house"}, // duplicate of correct — must be skipped
		{English: "tree"},
		{English: "car"},
		{English: "book"},
	}
	options, idx := buildMCOptions(correct, distractors, englishPicker)
	if len(options) != 4 {
		t.Fatalf("expected 4 options, got %d (likely failed to skip dup)", len(options))
	}
	if options[idx] != correct {
		t.Fatalf("correctIdx points to %q, expected %q", options[idx], correct)
	}
}

func TestModeLabel(t *testing.T) {
	if modeLabel(words.ModeEnRuMC) == "" {
		t.Fatalf("expected non-empty label for en_ru_mc")
	}
	if modeLabel(words.ModeRuEnTyping) == "" {
		t.Fatalf("expected non-empty label for ru_en_typing")
	}
}
