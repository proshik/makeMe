package assessment

import "makeMe/internal/domain/user"

// questionBank contains hardcoded questions per level for the initial assessment.
// Each level has enough questions for the adaptive algorithm to sample from.
var questionBank = map[user.Level][]Question{
	user.A1: {
		{Level: user.A1, Text: "She ___ a student.", Options: []string{"is", "are", "am", "be"}, CorrectOption: 0},
		{Level: user.A1, Text: "I ___ from Russia.", Options: []string{"is", "am", "are", "be"}, CorrectOption: 1},
		{Level: user.A1, Text: "They ___ happy.", Options: []string{"is", "am", "are", "be"}, CorrectOption: 2},
		{Level: user.A1, Text: "___ you like coffee?", Options: []string{"Does", "Do", "Is", "Are"}, CorrectOption: 1},
		{Level: user.A1, Text: "This is ___ apple.", Options: []string{"a", "an", "the", "—"}, CorrectOption: 1},
	},
	user.A2: {
		{Level: user.A2, Text: "She ___ to school every day.", Options: []string{"go", "goes", "going", "gone"}, CorrectOption: 1},
		{Level: user.A2, Text: "I ___ breakfast at 8 AM yesterday.", Options: []string{"have", "has", "had", "having"}, CorrectOption: 2},
		{Level: user.A2, Text: "There are ___ books on the table.", Options: []string{"much", "a lot", "some", "any"}, CorrectOption: 2},
		{Level: user.A2, Text: "He is ___ than his brother.", Options: []string{"tall", "taller", "tallest", "more tall"}, CorrectOption: 1},
		{Level: user.A2, Text: "We ___ TV when the phone rang.", Options: []string{"watch", "watched", "were watching", "are watching"}, CorrectOption: 2},
	},
	user.B1: {
		{Level: user.B1, Text: "If it rains, I ___ at home.", Options: []string{"stay", "will stay", "would stay", "stayed"}, CorrectOption: 1},
		{Level: user.B1, Text: "She has lived here ___ 2010.", Options: []string{"for", "since", "from", "during"}, CorrectOption: 1},
		{Level: user.B1, Text: "The book ___ by millions of people.", Options: []string{"has read", "has been read", "have read", "was reading"}, CorrectOption: 1},
		{Level: user.B1, Text: "I wish I ___ more free time.", Options: []string{"have", "had", "has", "having"}, CorrectOption: 1},
		{Level: user.B1, Text: "You ___ finish the report by Friday.", Options: []string{"must", "can", "might", "would"}, CorrectOption: 0},
	},
	user.B2: {
		{Level: user.B2, Text: "Had I known earlier, I ___ differently.", Options: []string{"would act", "would have acted", "acted", "had acted"}, CorrectOption: 1},
		{Level: user.B2, Text: "Not until she spoke ___ I realize my mistake.", Options: []string{"do", "did", "had", "was"}, CorrectOption: 1},
		{Level: user.B2, Text: "The project is worth ___.", Options: []string{"to pursue", "pursuing", "pursue", "pursued"}, CorrectOption: 1},
		{Level: user.B2, Text: "She denied ___ the document.", Options: []string{"to sign", "signing", "sign", "signed"}, CorrectOption: 1},
		{Level: user.B2, Text: "By next year, they ___ the bridge.", Options: []string{"will finish", "will have finished", "finish", "are finishing"}, CorrectOption: 1},
	},
	user.C1: {
		{Level: user.C1, Text: "Scarcely had he arrived ___ it started raining.", Options: []string{"than", "when", "then", "as"}, CorrectOption: 1},
		{Level: user.C1, Text: "The CEO, ___ decision was controversial, resigned.", Options: []string{"who", "whom", "whose", "which"}, CorrectOption: 2},
		{Level: user.C1, Text: "She ___ have left already; her coat is gone.", Options: []string{"must", "can", "should", "would"}, CorrectOption: 0},
		{Level: user.C1, Text: "Under no circumstances ___ this information.", Options: []string{"you should share", "should you share", "you share", "share you"}, CorrectOption: 1},
		{Level: user.C1, Text: "The results were ___ than we had anticipated.", Options: []string{"far more promising", "much more promisingly", "far most promising", "very more promising"}, CorrectOption: 0},
	},
	user.C2: {
		{Level: user.C2, Text: "The policy, albeit well-intentioned, proved ___.", Options: []string{"counterproductive", "counterproductively", "counterproduction", "counterproduct"}, CorrectOption: 0},
		{Level: user.C2, Text: "So ___ was the damage that the building was condemned.", Options: []string{"extensive", "extending", "extent", "extensible"}, CorrectOption: 0},
		{Level: user.C2, Text: "He speaks with such eloquence as to ___ even his critics.", Options: []string{"disarm", "disarming", "disarmed", "have disarming"}, CorrectOption: 0},
		{Level: user.C2, Text: "Were it not for her ___ intervention, the deal would have collapsed.", Options: []string{"timely", "timeless", "timing", "timed"}, CorrectOption: 0},
		{Level: user.C2, Text: "The phenomenon ___ explanation by conventional theories.", Options: []string{"defied", "defies", "defying", "to defy"}, CorrectOption: 1},
	},
}
