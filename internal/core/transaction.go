package core

import "time"

type Transaction struct {
	ID            int
	AccountID     int
	Type          string
	Amount        float64
	Timestamp     time.Time
	Reference     string
	FromAccountID *int
	ToAccountID   *int
}
