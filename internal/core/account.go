package core

import "time"

type Account struct {
	ID        int
	Name      string
	Balance   int64
	CreatedAt time.Time
}
