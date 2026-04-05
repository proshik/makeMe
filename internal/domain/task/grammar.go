package task

import (
	"context"
	"math/rand/v2"

	"makeMe/internal/domain/user"
)

// grammarQuestion is a hardcoded grammar exercise.
type grammarQuestion struct {
	Level   user.Level
	Prompt  string
	Options []string
	Answer  string
}

var grammarBank = map[user.Level][]grammarQuestion{
	user.A1: {
		{user.A1, "She ___ a teacher.", []string{"is", "are", "am", "be"}, "is"},
		{user.A1, "We ___ students.", []string{"is", "am", "are", "be"}, "are"},
		{user.A1, "He ___ a car.", []string{"have", "has", "having", "haves"}, "has"},
		{user.A1, "I ___ English every day.", []string{"study", "studies", "studying", "studied"}, "study"},
		{user.A1, "The cat ___ on the table.", []string{"sit", "sits", "sitting", "sitted"}, "sits"},
	},
	user.A2: {
		{user.A2, "She ___ to the store yesterday.", []string{"go", "goes", "went", "gone"}, "went"},
		{user.A2, "I ___ my homework right now.", []string{"do", "does", "am doing", "did"}, "am doing"},
		{user.A2, "They ___ already ___ lunch.", []string{"have...eaten", "has...eaten", "have...ate", "had...eat"}, "have...eaten"},
		{user.A2, "The movie was ___ than the book.", []string{"good", "better", "best", "more good"}, "better"},
		{user.A2, "You ___ turn off the lights when you leave.", []string{"should", "shall", "would", "could"}, "should"},
	},
	user.B1: {
		{user.B1, "If I ___ rich, I would travel.", []string{"am", "was", "were", "be"}, "were"},
		{user.B1, "The letter ___ yesterday.", []string{"sent", "was sent", "is sent", "sends"}, "was sent"},
		{user.B1, "She told me that she ___ busy.", []string{"is", "was", "were", "been"}, "was"},
		{user.B1, "I've been working here ___ five years.", []string{"since", "for", "from", "during"}, "for"},
		{user.B1, "He ___ have arrived by now.", []string{"must", "can", "shall", "would"}, "must"},
	},
	user.B2: {
		{user.B2, "Had she studied harder, she ___ the exam.", []string{"passed", "would pass", "would have passed", "had passed"}, "would have passed"},
		{user.B2, "The report ___ by the time you arrive.", []string{"will finish", "will be finished", "will have been finished", "finishes"}, "will have been finished"},
		{user.B2, "Not only ___ he apologize, but he also paid.", []string{"does", "did", "had", "was"}, "did"},
		{user.B2, "I regret ___ you earlier.", []string{"not telling", "not to tell", "to not tell", "not tell"}, "not telling"},
		{user.B2, "It's high time we ___ a decision.", []string{"make", "made", "making", "have made"}, "made"},
	},
	user.C1: {
		{user.C1, "Little ___ he know what awaited him.", []string{"does", "did", "had", "was"}, "did"},
		{user.C1, "The evidence ___ to suggest a different conclusion.", []string{"seems", "is seeming", "seem", "has seeming"}, "seems"},
		{user.C1, "Were he ___ the truth, things would be different.", []string{"to tell", "telling", "told", "tell"}, "to tell"},
		{user.C1, "She left early ___ miss the traffic.", []string{"so as not to", "for not to", "so that not", "in order not"}, "so as not to"},
		{user.C1, "Seldom ___ such a performance.", []string{"have I seen", "I have seen", "I saw", "did I saw"}, "have I seen"},
	},
	user.C2: {
		{user.C2, "Be that ___ it may, we must proceed.", []string{"as", "so", "what", "how"}, "as"},
		{user.C2, "So pervasive ___ the influence that none escaped it.", []string{"was", "is", "were", "had"}, "was"},
		{user.C2, "He spoke ___ he had witnessed the event firsthand.", []string{"as though", "like if", "as like", "so that"}, "as though"},
		{user.C2, "The committee ___ yet to reach a consensus.", []string{"has", "have", "had", "is"}, "has"},
		{user.C2, "Try ___ he might, success eluded him.", []string{"as", "though", "hard", "much"}, "as"},
	},
}

// GrammarGenerator generates grammar fill-in-the-blank tasks.
type GrammarGenerator struct{}

func NewGrammarGenerator() *GrammarGenerator {
	return &GrammarGenerator{}
}

func (g *GrammarGenerator) Type() TaskType { return TypeGrammar }

func (g *GrammarGenerator) Generate(_ context.Context, _ int64, level user.Level, count int) ([]Task, error) {
	questions := grammarBank[level]
	if len(questions) == 0 {
		questions = grammarBank[user.B1]
	}

	// Shuffle and pick
	perm := rand.Perm(len(questions))
	tasks := make([]Task, 0, count)
	for i := 0; i < count && i < len(perm); i++ {
		q := questions[perm[i]]
		tasks = append(tasks, Task{
			Type:    TypeGrammar,
			Level:   q.Level,
			Prompt:  q.Prompt,
			Options: q.Options,
			Answer:  q.Answer,
		})
	}

	return tasks, nil
}
