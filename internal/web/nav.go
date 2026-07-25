package web

import "makeMe/internal/domain/user"

// Section identifies the primary nav item a page belongs to, so the chrome can
// show where the user currently is. An empty value means "no primary item is
// current" — the profile and groups pages live under the account menu, not in
// the primary rail.
type Section string

const (
	SectionTasks       Section = "tasks"
	SectionWords       Section = "words"
	SectionLearn       Section = "learn"
	SectionLeaderboard Section = "leaderboard"
	SectionAccount     Section = "account"
)

// chrome is embedded by every page struct so one nav template can render the
// header without each handler assembling it. Fields promote, so a page can
// still be addressed as .User in its own template.
type chrome struct {
	User    *user.User
	Section Section
}

func tasksChrome(u *user.User) chrome { return chrome{User: u, Section: SectionTasks} }
func wordsChrome(u *user.User) chrome { return chrome{User: u, Section: SectionWords} }
