package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
)

type MailSender interface {
	Send(subject, body string, to []string) error
}

type TaskProcessor interface {
	Start() error
	Shutdown()
	ProcessTaskSendEmail(ctx context.Context, task *asynq.Task) error
}

type RedisTaskProcessor struct {
	server *asynq.Server
	mailer MailSender
	logger *slog.Logger
}

func NewRedisTaskProcessor(redisOpt asynq.RedisClientOpt, mailer MailSender, logger *slog.Logger) TaskProcessor {
	server := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Queues: map[string]int{
				"default":  10,
				"critical": 20,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				logger.Error("process task failed",
					"type", task.Type(),
					"payload", string(task.Payload()),
					"error", err,
				)
			}),
			Logger: NewLoggerAdapter(logger),
		},
	)

	return &RedisTaskProcessor{
		server: server,
		mailer: mailer,
		logger: logger,
	}
}

func (processor *RedisTaskProcessor) Start() error {
	mux := asynq.NewServeMux()

	mux.HandleFunc(TaskSendEmail, processor.ProcessTaskSendEmail)

	return processor.server.Run(mux)
}

func (processor *RedisTaskProcessor) Shutdown() {
	processor.server.Shutdown()
}

func (processor *RedisTaskProcessor) ProcessTaskSendEmail(ctx context.Context, task *asynq.Task) error {
	var payload PayloadSendEmail
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		// SkipRetry because malformed payload won't be fixed by retrying
		return fmt.Errorf("failed to unmarshal payload: %w: %w", err, asynq.SkipRetry)
	}

	processor.logger.Info("sending email", "type", task.Type(), "email", payload.Email, "subject", payload.Subject)

	if err := processor.mailer.Send(payload.Subject, payload.Body, []string{payload.Email}); err != nil {
		return fmt.Errorf("failed to send email to %s: %w", payload.Email, err)
	}

	processor.logger.Info("email sent successfully", "email", payload.Email)
	return nil
}
