package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"makeMe/internal/domain/assessment"
	"makeMe/internal/domain/group"
	"makeMe/internal/domain/scoring"
	"makeMe/internal/domain/user"
)

// groupSummary holds a user's daily result for one group.
type groupSummary struct {
	GroupName  string
	Raw        int
	Normalized float64
	Rank       int
	Total      int
}

// Scheduler sends proactive messages to users at a configured daily time.
type Scheduler struct {
	userRepo    user.Repository
	groupSvc    *group.Service
	scoringSvc  *scoring.Service
	assessRepo  assessment.Repository
	sendMessage func(ctx context.Context, chatID int64, text string) error
	timezone    *time.Location
	dailyTime   string // "HH:MM"
}

func New(
	userRepo user.Repository,
	groupSvc *group.Service,
	scoringSvc *scoring.Service,
	assessRepo assessment.Repository,
	sendMessage func(ctx context.Context, chatID int64, text string) error,
	timezone *time.Location,
	dailyTime string,
) *Scheduler {
	return &Scheduler{
		userRepo:    userRepo,
		groupSvc:    groupSvc,
		scoringSvc:  scoringSvc,
		assessRepo:  assessRepo,
		sendMessage: sendMessage,
		timezone:    timezone,
		dailyTime:   dailyTime,
	}
}

// Run blocks and fires RunDailySummary once per day at the configured time.
func (s *Scheduler) Run(ctx context.Context) {
	for {
		next := nextRunTime(time.Now().In(s.timezone), s.dailyTime, s.timezone)
		slog.Info("scheduler: next daily summary", "at", next)

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
		}

		if err := s.RunDailySummary(ctx); err != nil {
			slog.Error("scheduler: daily summary failed", "error", err)
		}
	}
}

// RunDailySummary sends today's results to all users who belong to at least one group.
func (s *Scheduler) RunDailySummary(ctx context.Context) error {
	users, err := s.userRepo.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}

	today := time.Now().Format("2006-01-02")

	for _, u := range users {
		groups, err := s.groupSvc.ListByUser(ctx, u.ID)
		if err != nil {
			slog.Warn("scheduler: list groups failed", "userID", u.ID, "error", err)
			continue
		}
		if len(groups) == 0 {
			continue
		}

		var summaries []groupSummary
		for _, g := range groups {
			entries, err := s.scoringSvc.GetLeaderboard(ctx, g.ID, "day", today)
			if err != nil {
				slog.Warn("scheduler: leaderboard failed", "groupID", g.ID, "error", err)
				continue
			}
			for _, e := range entries {
				if e.UserID == u.ID {
					summaries = append(summaries, groupSummary{
						GroupName:  g.Name,
						Raw:        e.Raw,
						Normalized: e.Normalized,
						Rank:       e.Rank,
						Total:      len(entries),
					})
					break
				}
			}
		}

		msg := formatDailySummary(u.FirstName, summaries)
		if err := s.sendMessage(ctx, u.TelegramID, msg); err != nil {
			slog.Warn("scheduler: send message failed", "userID", u.ID, "error", err)
		}
	}
	return nil
}

// RunReassessReminder sends a re-assessment reminder to users whose last assessment
// was completed more than 30 days ago.
func (s *Scheduler) RunReassessReminder(ctx context.Context) error {
	users, err := s.userRepo.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -30)

	for _, u := range users {
		session, err := s.assessRepo.GetLastCompleted(ctx, u.ID)
		if err != nil {
			continue // no completed session — skip
		}
		if session.CompletedAt.After(cutoff) {
			continue // assessed recently — skip
		}

		msg := formatReassessReminder(u.FirstName)
		if err := s.sendMessage(ctx, u.TelegramID, msg); err != nil {
			slog.Warn("scheduler: reassess reminder failed", "userID", u.ID, "error", err)
		}
	}
	return nil
}

// nextRunTime returns the next wall-clock occurrence of dailyTime ("HH:MM") in loc,
// starting strictly after now.
func nextRunTime(now time.Time, dailyTime string, loc *time.Location) time.Time {
	var h, m int
	fmt.Sscanf(dailyTime, "%d:%d", &h, &m)

	candidate := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, loc)
	if !candidate.After(now) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate
}

func formatDailySummary(firstName string, summaries []groupSummary) string {
	if len(summaries) == 0 {
		return fmt.Sprintf(
			"Привет, %s!\n\nСегодня ты ещё не выполнял задания.\nЗагляни в /tasks — там ждут новые задания! 📚",
			firstName,
		)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 Итоги дня, %s!\n", firstName))

	for _, s := range summaries {
		sb.WriteString(fmt.Sprintf(
			"\n%s:\n  Баллы: %d (%.1f норм.)\n  Место: %d из %d\n",
			s.GroupName, s.Raw, s.Normalized, s.Rank, s.Total,
		))
	}

	sb.WriteString("\nПродолжай в том же духе! 🎯")
	return sb.String()
}

func formatReassessReminder(firstName string) string {
	return fmt.Sprintf(
		"Привет, %s! 👋\n\nПрошло больше месяца с твоего последнего теста уровня.\n"+
			"Попробуй пройти его снова — возможно, твой уровень вырос!\n\n"+
			"Нажми /assess чтобы начать.",
		firstName,
	)
}
