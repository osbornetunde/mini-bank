package worker

import (
	"log/slog"

	"github.com/hibiken/asynq"
)

type TaskDistributor interface {
	DistributeTaskSendEmail(
		payload *PayloadSendEmail,
		opts ...asynq.Option,
	) error
	Close() error
}

type RedisTaskDistributor struct {
	client *asynq.Client
	logger *slog.Logger
}

func NewRedisTaskDistributor(redisOpt asynq.RedisClientOpt, logger *slog.Logger) TaskDistributor {
	client := asynq.NewClient(redisOpt)
	return &RedisTaskDistributor{
		client: client,
		logger: logger,
	}
}

func (d *RedisTaskDistributor) Close() error {
	return d.client.Close()
}
