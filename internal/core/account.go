package core

import "time"

type Account struct {
	ID        int
	Name      string
	Balance   float64
	CreatedAt time.Time
}
