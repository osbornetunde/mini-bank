package worker

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

const TaskSendEmail = "task:send_email"

// PayloadSendEmail is the payload for the send email task.
// Used for both enqueuing and processing.
type PayloadSendEmail struct {
	Email   string `json:"email"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func (d *RedisTaskDistributor) DistributeTaskSendEmail(
	payload *PayloadSendEmail,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal task payload: %w", err)
	}

	task := asynq.NewTask(TaskSendEmail, jsonPayload, opts...)
	info, err := d.client.Enqueue(task)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	d.logger.Info("enqueued email task",
		"task_id", info.ID,
		"queue", info.Queue,
		"email", payload.Email,
	)
	return nil
}
