package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// parseTemplates builds the template set exactly as New does, without the
// services New needs, so a fragment can be rendered on its own.
func parseTemplates(t *testing.T) *template.Template {
	t.Helper()
	tpl, err := template.New("").Funcs(templateFuncs("test")).ParseFS(templateFS,
		"templates/*.html", "templates/fragments/*.html")
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}
	return tpl
}

const multiLineRule = "Форма: as if + прошедшее время\r\n\r\n" +
	"Нереальное (сейчас):\n" +
	"• She talks as if she knew everything.\n" +
	"  • He looks as if he were ill.  \n\n" +
	"Частая ошибка: <b>bold</b> & <script>alert(1)</script>"

func TestParagraphsSplitsLinesAndDropsBlanks(t *testing.T) {
	paragraphs := templateFuncs("test")["paragraphs"].(func(string) []string)

	got := paragraphs(multiLineRule)
	want := []string{
		"Форма: as if + прошедшее время",
		"Нереальное (сейчас):",
		"• She talks as if she knew everything.",
		"• He looks as if he were ill.",
		"Частая ошибка: <b>bold</b> & <script>alert(1)</script>",
	}
	if len(got) != len(want) {
		t.Fatalf("paragraphs = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("paragraphs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if len(paragraphs("")) != 0 || len(paragraphs(" \n\n ")) != 0 {
		t.Error("blank text must yield no paragraphs")
	}
}

// The rule after a wrong task answer once rendered as one <p>, so its
// headings and "•" items ran together into a wall of text.
func TestTaskFeedbackRendersRuleLineByLineEscaped(t *testing.T) {
	var buf bytes.Buffer
	err := parseTemplates(t).ExecuteTemplate(&buf, "task_feedback", feedbackView{
		Answer:      "were",
		Explanation: multiLineRule,
		HasNext:     true,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	for _, line := range []string{
		">Форма: as if &#43; прошедшее время</p>",
		">Нереальное (сейчас):</p>",
		">• She talks as if she knew everything.</p>",
		">• He looks as if he were ill.</p>",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("feedback lacks its own paragraph %q:\n%s", line, out)
		}
	}
	if strings.Contains(out, "<script>") || strings.Contains(out, "<b>bold") {
		t.Errorf("markup from the rule text reached the page unescaped:\n%s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("rule text was dropped instead of escaped:\n%s", out)
	}
}
