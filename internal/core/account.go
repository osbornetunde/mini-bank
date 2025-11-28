package core

import "time"

type Account struct {
	ID        int
	UserID    int
	Balance   int64
	CreatedAt time.Time
}
