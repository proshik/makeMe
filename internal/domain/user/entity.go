package user

import "time"

type Level string

const (
	A1 Level = "A1"
	A2 Level = "A2"
	B1 Level = "B1"
	B2 Level = "B2"
	C1 Level = "C1"
	C2 Level = "C2"
)

type User struct {
	ID         int64
	TelegramID int64
	Username   string
	FirstName  string
	Level      Level
	Onboarded  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
