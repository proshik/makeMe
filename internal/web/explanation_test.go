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
	"• She talks as if she knew everything. — Она говорит так, будто всё знает.\n" +
	"  • He looks as if he were ill.  \n\n" +
	"Частые ошибки:\n" +
	"• ✗ He looks as if he is ill. → ✓ He looks as if he were ill.\n\n" +
	"Частая ошибка: <b>bold</b> & <script>alert(1)</script>"

// The rule after a wrong task answer once rendered as one <p>, and later as a
// flat run of same-coloured lines. It now comes in blocks: a lead, a plate per
// section with its heading, examples split into English and translation,
// mistakes split into the wrong and the right form.
func TestTaskFeedbackRendersRuleInBlocksEscaped(t *testing.T) {
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

	for _, want := range []string{
		`<p class="ruledoc-lead"><span class="ruledoc-label">Форма</span><span class="ruledoc-text">as if &#43; прошедшее время</span></p>`,
		`<h3 class="ruledoc-h">Нереальное (сейчас)</h3>`,
		`<span class="ruledoc-en" lang="en">She talks as if she knew everything.</span><span class="ruledoc-ru">Она говорит так, будто всё знает.</span>`,
		`<span class="ruledoc-text">He looks as if he were ill.</span>`,
		`<s lang="en">He looks as if he is ill.</s>`,
		`<span class="ruledoc-right"><span class="sr-only">Верно: </span><span lang="en">He looks as if he were ill.</span></span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("feedback lacks %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, `class="ruledoc-sec"`); n != 3 {
		t.Errorf("sections = %d, want 3", n)
	}
	// The last block opens with a captioned line: its caption is a heading.
	if want := `<h3 class="ruledoc-h">Частая ошибка</h3>`; !strings.Contains(out, want) {
		t.Errorf("feedback lacks %q:\n%s", want, out)
	}
	if strings.Contains(out, "<script>") || strings.Contains(out, "<b>bold") {
		t.Errorf("markup from the rule text reached the page unescaped:\n%s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("rule text was dropped instead of escaped:\n%s", out)
	}
}

// Topic page and both feedbacks draw the rule with the same component.
func TestRuleComponentSharedByAllThreePlaces(t *testing.T) {
	tpl := parseTemplates(t)
	render := func(name string, data any) string {
		var buf bytes.Buffer
		if err := tpl.ExecuteTemplate(&buf, name, data); err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		return buf.String()
	}
	for name, out := range map[string]string{
		"learn_topic.html": render("learn_topic.html", learnTopicPage{Topic: "as if", Explanation: multiLineRule, Total: 3}),
		"learn_feedback":   render("learn_feedback", learnFeedback{Topic: "as if", Explanation: multiLineRule}),
		"task_feedback":    render("task_feedback", feedbackView{Explanation: multiLineRule}),
	} {
		if !strings.Contains(out, `<div class="ruledoc">`) || !strings.Contains(out, `<h3 class="ruledoc-h">Частые ошибки</h3>`) {
			t.Errorf("%s does not render the rule component:\n%s", name, out)
		}
	}

	// No explanation, no empty frame.
	if out := render("task_feedback", feedbackView{}); strings.Contains(out, "ruledoc") {
		t.Errorf("feedback without a rule renders a rule frame:\n%s", out)
	}
	if out := render("learn_topic.html", learnTopicPage{Topic: "x", Total: 1}); !strings.Contains(out, "Описание для этой темы пока не добавлено") {
		t.Errorf("topic without theory lacks the placeholder:\n%s", out)
	}
}
