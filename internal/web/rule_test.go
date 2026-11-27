package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestParseRuleSplitsLeadSectionsAndLineKinds(t *testing.T) {
	doc := parseRule("Глагол to be в настоящем времени.\n\n" +
		"Когда использовать:\n" +
		"• Описать кого-то: She is a doctor.\n" +
		"• Предложение: I can help. — Я могу помочь.\n\n" +
		"Примеры:\n" +
		"• Take an umbrella — it's raining. — Возьми зонт — идёт дождь.\n" +
		"• He is a good player. — Он хороший игрок. (прилагательное)\n\n" +
		"Частые ошибки:\n" +
		"• ✗ He always is late. → ✓ He is always late. (после be!)\n" +
		"• ✓ Feeling tired, she sat down. (она устала — она села)\n\n" +
		"Прилагательные на -ly (НЕ наречия): friendly, lovely.\n" +
		"• ✗ She smiled friendlily. → ✓ She gave me a friendly smile.\n\n" +
		"Таблица:\n" +
		"I      | me    | my\n")

	if len(doc.Lead) != 1 || doc.Lead[0].Text != "Глагол to be в настоящем времени." {
		t.Fatalf("lead = %+v", doc.Lead)
	}
	if len(doc.Sections) != 5 {
		t.Fatalf("sections = %d, want 5: %+v", len(doc.Sections), doc.Sections)
	}

	use := doc.Sections[0]
	if use.Heading != "Когда использовать" {
		t.Errorf("heading = %q", use.Heading)
	}
	if l := use.Lines[0]; !l.Bullet || l.Label != "Описать кого-то" || l.Kind != ruleText || l.Text != "She is a doctor." {
		t.Errorf("labelled item = %+v", l)
	}
	if l := use.Lines[1]; l.Label != "Предложение" || l.Kind != ruleExample ||
		l.English != "I can help." || l.Russian != "Я могу помочь." {
		t.Errorf("labelled example = %+v", l)
	}

	ex := doc.Sections[1].Lines
	if ex[0].English != "Take an umbrella — it's raining." || ex[0].Russian != "Возьми зонт — идёт дождь." {
		t.Errorf("an English dash must stay with the English side: %+v", ex[0])
	}
	if ex[1].Russian != "Он хороший игрок. (прилагательное)" {
		t.Errorf("example = %+v", ex[1])
	}

	mis := doc.Sections[2].Lines
	if l := mis[0]; l.Kind != ruleMistake || l.Wrong != "He always is late." ||
		l.Right != "He is always late." || l.Note != "(после be!)" {
		t.Errorf("mistake = %+v", l)
	}
	if l := mis[1]; l.Kind != ruleMistake || l.Wrong != "" || l.Right != "Feeling tired, she sat down." {
		t.Errorf("right-only line = %+v", l)
	}

	// A block opening with a captioned line gets the caption as its heading;
	// the rest of that line goes under it, the list follows.
	capt := doc.Sections[3]
	if capt.Heading != "" || capt.Caption == nil || capt.Caption.Label != "Прилагательные на -ly (НЕ наречия)" ||
		capt.CaptionBody().Text != "friendly, lovely." || capt.CaptionBody().Label != "" ||
		len(capt.Lines) != 1 || capt.Lines[0].Kind != ruleMistake {
		t.Errorf("block opening with a captioned line = %+v", capt)
	}
	// Captioned items inside a list stay items.
	if use.Caption != nil || use.Lines[0].Label == "" {
		t.Errorf("a captioned list item must not become a caption: %+v", use)
	}

	if l := doc.Sections[4].Lines[0]; l.Kind != ruleTable || l.Text != "I      | me    | my" {
		t.Errorf("table row must keep its spacing: %+v", l)
	}
}

// Lines that look almost like a convention stay plain text: a half-parsed
// line would draw part of it in the wrong colour or drop the rest.
func TestParseRuleLeavesAmbiguousLinesAsText(t *testing.T) {
	for _, line := range []string{
		"• НО: I've got a car. (владею — НЕ Continuous)", // dash inside a Russian remark
		"• this (ед.ч., близко) — этот/эта/это: This book is mine.",
		"• ✗ wrong → but no right mark",
		"• В коротких ответах: Who did it? — Me!", // dash, but no Russian after it
		"• go → went (идти / ехать)",
	} {
		l := parseRuleLine(line)
		if l.Kind != ruleText {
			t.Errorf("%q parsed as %s: %+v", line, l.Kind, l)
		}
		if l.String() != line {
			t.Errorf("%q rebuilt as %q", line, l.String())
		}
	}
}

// Only the first line of a block can be a caption, and only a plain one: an
// unlabelled opening line or a bulleted one stays a list line, and the lead
// paragraph keeps its own captions.
func TestParseRuleCaptionOnlyOpensABlock(t *testing.T) {
	doc := parseRule("Форма: as if + Past Simple\n\n" +
		"Present Simple: don't / doesn't + V\n" +
		"• I don't like coffee.\n" +
		"Подсказка: не в начале блока\n\n" +
		"• Пункт: с подписью первым\n\n" +
		"Просто строка без подписи\n\n" +
		"Частая ошибка: ✗ Turn on it. → ✓ Turn it on.")

	if len(doc.Lead) != 1 || doc.Lead[0].Label != "Форма" {
		t.Errorf("lead = %+v", doc.Lead)
	}
	if len(doc.Sections) != 4 {
		t.Fatalf("sections = %+v", doc.Sections)
	}
	ps := doc.Sections[0]
	if ps.Caption == nil || ps.Caption.Label != "Present Simple" || ps.CaptionBody().Text != "don't / doesn't + V" {
		t.Errorf("caption = %+v", ps)
	}
	if len(ps.Lines) != 2 || ps.Lines[1].Label != "Подсказка" {
		t.Errorf("a captioned line later in the block stays a line: %+v", ps.Lines)
	}
	for i, sec := range doc.Sections[1:3] {
		if sec.Caption != nil {
			t.Errorf("section %d must not get a caption: %+v", i+1, sec)
		}
	}
	if m := doc.Sections[3]; m.Caption == nil || m.Caption.Label != "Частая ошибка" || !m.CaptionBody().IsMistake() {
		t.Errorf("captioned mistake = %+v", m)
	}
}

func TestParseRuleEmpty(t *testing.T) {
	for _, s := range []string{"", " \n\n \r\n"} {
		if !parseRule(s).Empty() {
			t.Errorf("parseRule(%q) is not empty", s)
		}
	}
}

// normaliseRuleLines is the source as the parser sees it: trimmed, blank
// lines dropped, "•" followed by exactly one space.
func normaliseRuleLines(text string) []string {
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, bulletMark) {
			l = bulletMark + " " + strings.TrimSpace(strings.TrimPrefix(l, bulletMark))
		}
		out = append(out, l)
	}
	return out
}

func assertRuleLossless(t *testing.T, name, text string) ruleDoc {
	t.Helper()
	doc := parseRule(text)
	want := normaliseRuleLines(text)
	got := normaliseRuleLines(doc.String())
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s: parse lost or changed text\nwant:\n%s\ngot:\n%s",
			name, strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
	return doc
}

func repoFile(t *testing.T, rel string) []byte {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "..", "..", rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return b
}

// Every rule the app ships must survive the parse intact, and the
// conventions must actually be recognised in it — a parser that turned
// everything into plain text would pass the first check alone.
func TestParseRuleKeepsEveryShippedExplanation(t *testing.T) {
	var cats []struct {
		Name        string `json:"name"`
		Explanation string `json:"explanation"`
	}
	if err := json.Unmarshal(repoFile(t, "data/grammar_categories.json"), &cats); err != nil {
		t.Fatalf("parse grammar_categories.json: %v", err)
	}
	if len(cats) == 0 {
		t.Fatal("no categories")
	}

	counts := map[ruleLineKind]int{}
	unparsedMistakes := []string{}
	for _, c := range cats {
		doc := assertRuleLossless(t, c.Name, c.Explanation)
		if len(doc.Lead) == 0 {
			t.Errorf("%s: no lead paragraph", c.Name)
		}
		for _, s := range doc.Sections {
			lines := s.Lines
			if s.Caption != nil {
				counts["caption"]++
				lines = append([]ruleLine{*s.Caption}, lines...)
			}
			for _, l := range lines {
				counts[l.Kind]++
				// "✗ … → Если сам, то: …" mixes notations on purpose and
				// stays text; every other ✗ line must be recognised.
				b := l.body()
				if l.Kind != ruleMistake && strings.HasPrefix(b, wrongMark) &&
					(!strings.Contains(b, arrowMark) || strings.Contains(b, arrowMark+" "+rightMark)) {
					unparsedMistakes = append(unparsedMistakes, c.Name+": "+l.String())
				}
			}
		}
	}
	if len(unparsedMistakes) > 0 {
		t.Errorf("mistake lines left as text:\n%s", strings.Join(unparsedMistakes, "\n"))
	}
	for _, k := range []ruleLineKind{ruleExample, ruleMistake, ruleTable, ruleText, "caption"} {
		if counts[k] == 0 {
			t.Errorf("no %s lines recognised across all categories", k)
		}
	}
	t.Logf("line kinds: %v", counts)

	// Questions carry no explanation of their own today — they show their
	// category's. If one ever gets its own text, it must survive too.
	var questions []map[string]any
	if err := json.Unmarshal(repoFile(t, "data/grammar.json"), &questions); err != nil {
		t.Fatalf("parse grammar.json: %v", err)
	}
	for i, q := range questions {
		if e, _ := q["explanation"].(string); e != "" {
			assertRuleLossless(t, "grammar.json #"+strconv.Itoa(i), e)
		}
	}
}
