package core

import "time"

type AuditLog struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Action    string    `json:"action"`
	Metadata  string    `json:"metadata"` // JSON string for flexibility
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
}
