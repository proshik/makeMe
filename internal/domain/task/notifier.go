package task

import "context"

// AnswerEvent describes an answer that was just recorded, whichever channel it
// came from. Scored is false for wrong answers and skips — the leaderboard
// cannot have moved in that case, so listeners can skip broadcasting it.
type AnswerEvent struct {
	UserID  int64
	GroupID int64
	Scored  bool
}

// Notifier is told about answers so other open sessions can catch up. The
// service calls it from SubmitAnswer and friends, which is the one place both
// the bot and the web funnel through — publishing from the adapters instead
// would mean two call sites that drift apart.
//
// Implementations must not block: the service calls this on the request path.
type Notifier interface {
	TaskAnswered(ctx context.Context, ev AnswerEvent)
}

// notify is nil-safe so the service works without a notifier — that is the
// normal state in tests and in a bot-only deployment.
func (s *Service) notify(ctx context.Context, ev AnswerEvent) {
	if s.notifier == nil {
		return
	}
	s.notifier.TaskAnswered(ctx, ev)
}
