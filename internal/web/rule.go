package web

import (
	"strings"
	"unicode"
)

// ruleDoc is a grammar explanation split into the parts the page draws
// differently: an opening paragraph, then one plate per section, each line
// typed so an example can show its translation dimmed and a mistake can show
// the wrong form in red next to the right one.
//
// Every field is plain text. The template escapes it; nothing from the data
// ever becomes markup.
//
// The stored text is the source of truth and the parser only reads its
// conventions — blank line between blocks, a heading ending in ":", items
// starting with "•", "EN — RU" examples, "✗ wrong → ✓ right" mistakes. A line
// that does not fit a convention exactly is kept as plain text rather than
// half-parsed, so nothing a content author wrote can disappear. String()
// rebuilds the source from the parts, which is how the tests prove that.
type ruleDoc struct {
	Lead     []ruleLine
	Sections []ruleSection
}

type ruleSection struct {
	Heading string // without the trailing colon; empty for a block that opens with an ordinary line
	Lines   []ruleLine
}

type ruleLineKind string

const (
	ruleText    ruleLineKind = "text"
	ruleExample ruleLineKind = "example"
	ruleMistake ruleLineKind = "mistake"
	ruleTable   ruleLineKind = "table"
)

type ruleLine struct {
	Kind   ruleLineKind
	Bullet bool
	// Label is the caption before the content: "После глагола" in
	// "После глагола: She called me. — Она позвонила мне.". No colon.
	Label string

	Text string // ruleText, ruleTable

	English string // ruleExample
	Russian string

	Wrong string // ruleMistake; either side may be empty, not both
	Right string
	Note  string // trailing Russian remark in parentheses after the right form
}

func (l ruleLine) IsText() bool    { return l.Kind == ruleText }
func (l ruleLine) IsExample() bool { return l.Kind == ruleExample }
func (l ruleLine) IsMistake() bool { return l.Kind == ruleMistake }
func (l ruleLine) IsTable() bool   { return l.Kind == ruleTable }

// Empty reports whether there is nothing to draw, so callers can fall back to
// "no description yet" without inspecting the parts.
func (d ruleDoc) Empty() bool { return len(d.Lead) == 0 && len(d.Sections) == 0 }

const (
	bulletMark  = "•"
	wrongMark   = "✗"
	rightMark   = "✓"
	arrowMark   = "→"
	exampleSep  = " — "
	labelSep    = ": "
	maxLabelLen = 60 // runes; anything longer is a sentence with a colon, not a caption
)

// parseRule splits stored theory into a ruleDoc. Blocks are separated by blank
// lines; the first block is the lead unless it opens with a heading.
func parseRule(text string) ruleDoc {
	var doc ruleDoc
	text = strings.ReplaceAll(text, "\r\n", "\n")

	blockIndex := -1
	var cur *ruleSection
	prevBlank := true
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			prevBlank = true
			continue
		}
		if prevBlank {
			blockIndex++
			cur = nil
			prevBlank = false
		}

		if heading, ok := ruleHeading(line); ok {
			doc.Sections = append(doc.Sections, ruleSection{Heading: heading})
			cur = &doc.Sections[len(doc.Sections)-1]
			continue
		}

		parsed := parseRuleLine(line)
		switch {
		case cur != nil:
			cur.Lines = append(cur.Lines, parsed)
		case blockIndex == 0:
			doc.Lead = append(doc.Lead, parsed)
		default:
			doc.Sections = append(doc.Sections, ruleSection{Lines: []ruleLine{parsed}})
			cur = &doc.Sections[len(doc.Sections)-1]
		}
	}
	return doc
}

// ruleHeading recognises "Форма:" — a line that is not an item and ends in a
// colon. A bare ":" is not a heading.
func ruleHeading(line string) (string, bool) {
	if strings.HasPrefix(line, bulletMark) || !strings.HasSuffix(line, ":") {
		return "", false
	}
	h := strings.TrimSuffix(line, ":")
	return h, strings.TrimSpace(h) != ""
}

func parseRuleLine(line string) ruleLine {
	out := ruleLine{Kind: ruleText}
	body := line
	if strings.HasPrefix(body, bulletMark) {
		out.Bullet = true
		body = strings.TrimSpace(strings.TrimPrefix(body, bulletMark))
	}

	if strings.Count(body, "|") >= 2 {
		out.Kind, out.Text = ruleTable, body
		return out
	}

	if label, rest, ok := splitLabel(body); ok {
		out.Label = label
		body = rest
	}

	if l, ok := parseMistake(body); ok {
		l.Bullet, l.Label = out.Bullet, out.Label
		return l
	}
	if en, ru, ok := splitExample(body); ok {
		out.Kind, out.English, out.Russian = ruleExample, en, ru
		return out
	}
	out.Text = body
	return out
}

// splitLabel finds a caption in front of the content. A colon right before a
// mistake wins ("Частая ошибка: ✗ …"), otherwise the first ": ". The caption
// must be short and must not itself be an example ("by — к: by Friday" is an
// example whose translation happens to contain a colon).
func splitLabel(body string) (label, rest string, ok bool) {
	i := strings.Index(body, labelSep+wrongMark)
	if i < 0 {
		i = strings.Index(body, labelSep)
	}
	if i <= 0 {
		return "", body, false
	}
	label, rest = body[:i], strings.TrimSpace(body[i+len(labelSep):])
	if rest == "" || strings.Contains(label, exampleSep) || strings.Contains(label, wrongMark) ||
		strings.Contains(label, rightMark) || len([]rune(label)) > maxLabelLen {
		return "", body, false
	}
	// Only a caption the content's own spacing reproduces: "a:  b" would
	// otherwise come back as "a: b".
	if label+labelSep+rest != body {
		return "", body, false
	}
	return label, rest, true
}

// parseMistake reads "✗ wrong → ✓ right (примечание)", the wrong-only
// "✗ wrong (примечание)" and the right-only "✓ right (примечание)". Anything
// else is not a mistake line.
func parseMistake(body string) (ruleLine, bool) {
	l := ruleLine{Kind: ruleMistake}
	var right string
	switch {
	case strings.HasPrefix(body, wrongMark+" "):
		rest := strings.TrimPrefix(body, wrongMark+" ")
		i := strings.Index(rest, " "+arrowMark+" "+rightMark+" ")
		if i < 0 {
			// Wrong-only: "✗ She hates to wait. (возможно, но с -ing привычнее)".
			if strings.Contains(rest, arrowMark) || strings.Contains(rest, wrongMark) ||
				strings.Contains(rest, rightMark) {
				return ruleLine{}, false
			}
			l.Wrong, l.Note = splitNote(rest)
			if l.Wrong == "" || l.body() != body {
				return ruleLine{}, false
			}
			return l, true
		}
		l.Wrong = rest[:i]
		right = rest[i+len(" "+arrowMark+" "+rightMark+" "):]
		if l.Wrong == "" || strings.Contains(l.Wrong, wrongMark) {
			return ruleLine{}, false
		}
	case strings.HasPrefix(body, rightMark+" "):
		right = strings.TrimPrefix(body, rightMark+" ")
	default:
		return ruleLine{}, false
	}
	// A second right form is allowed ("✓ … / ✓ …"), a second mistake is not.
	if right == "" || strings.Contains(right, wrongMark) {
		return ruleLine{}, false
	}
	l.Right, l.Note = splitNote(right)
	if l.Right == "" || l.body() != body {
		return ruleLine{}, false
	}
	return l, true
}

// splitNote separates a trailing Russian remark: "He is always late. (после
// be!)". An English parenthesis stays part of the sentence.
func splitNote(s string) (text, note string) {
	if !strings.HasSuffix(s, ")") {
		return s, ""
	}
	i := strings.LastIndex(s, " (")
	if i <= 0 || !hasCyrillic(s[i:]) {
		return s, ""
	}
	return s[:i], s[i+1:]
}

// splitExample reads "English sentence. — Русский перевод." The separator is
// the first " — " followed by Russian text, so an English dash inside the
// sentence ("Take an umbrella — it's raining. — Возьми зонт") stays with the
// English side. The English side must carry no Cyrillic at all: otherwise the
// dash belongs to a Russian remark, not to a translation.
func splitExample(body string) (en, ru string, ok bool) {
	from := 0
	for {
		i := strings.Index(body[from:], exampleSep)
		if i < 0 {
			return "", "", false
		}
		i += from
		en, ru = body[:i], body[i+len(exampleSep):]
		if startsCyrillic(ru) {
			if hasCyrillic(en) || !hasLatin(en) {
				return "", "", false
			}
			return en, ru, true
		}
		from = i + len(exampleSep)
	}
}

func (l ruleLine) body() string {
	switch l.Kind {
	case ruleExample:
		return l.English + exampleSep + l.Russian
	case ruleMistake:
		var s string
		switch {
		case l.Wrong != "" && l.Right != "":
			s = wrongMark + " " + l.Wrong + " " + arrowMark + " " + rightMark + " " + l.Right
		case l.Wrong != "":
			s = wrongMark + " " + l.Wrong
		default:
			s = rightMark + " " + l.Right
		}
		if l.Note != "" {
			s += " " + l.Note
		}
		return s
	default:
		return l.Text
	}
}

// String rebuilds the line in the stored notation.
func (l ruleLine) String() string {
	var b strings.Builder
	if l.Bullet {
		b.WriteString(bulletMark + " ")
	}
	if l.Label != "" {
		b.WriteString(l.Label + labelSep)
	}
	b.WriteString(l.body())
	return b.String()
}

// String rebuilds the explanation in the stored notation. Blank lines between
// blocks are not guaranteed to match the source; every other line is.
func (d ruleDoc) String() string {
	var blocks []string
	var lead []string
	for _, l := range d.Lead {
		lead = append(lead, l.String())
	}
	if len(lead) > 0 {
		blocks = append(blocks, strings.Join(lead, "\n"))
	}
	for _, s := range d.Sections {
		var lines []string
		if s.Heading != "" {
			lines = append(lines, s.Heading+":")
		}
		for _, l := range s.Lines {
			lines = append(lines, l.String())
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func startsCyrillic(s string) bool {
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Cyrillic, r):
			return true
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			return false
		}
		// Skip leading quotes and brackets: «Я», (уже).
	}
	return false
}

func hasCyrillic(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Cyrillic, r) {
			return true
		}
	}
	return false
}

func hasLatin(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Latin, r) {
			return true
		}
	}
	return false
}
