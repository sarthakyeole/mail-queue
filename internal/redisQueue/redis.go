package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sarthakyeole/redis-go-mailing-bulk/internal/config"
	email "github.com/sarthakyeole/redis-go-mailing-bulk/internal/senderSide"
)

const (
	emailQueue = "email_queue"

	maxRetries         = 3
	retryDelay         = 5 * time.Second
	queueCheckInterval = 1 * time.Second
)

type EmailTask struct {
	To           string                 `json:"to"`
	Subject      string                 `json:"subject"`
	TemplateName string                 `json:"templateName"`
	Data         map[string]interface{} `json:"data"`
	Retries      int                    `json:"retries,omitempty"`
}

type RedisQueue struct {
	client *redis.Client
	sender *email.Sender
	logger *slog.Logger
}

func NewRedisClient(cfg *config.ApplicationConfig) (*redis.Client, error) {
	if err := validateRedisConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid Redis configuration: %w", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.CacheHost, cfg.CachePort),
		Password: cfg.CachePassword,
		DB:       cfg.CacheDatabaseIndex,

		PoolSize:           10,
		PoolTimeout:        30 * time.Second,
		IdleCheckFrequency: 5 * time.Minute,
		IdleTimeout:        5 * time.Minute,
		MaxConnAge:         30 * time.Minute,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return client, nil
}

func validateRedisConfig(cfg *config.ApplicationConfig) error {
	if cfg.CacheHost == "" {
		return fmt.Errorf("redis host cannot be empty")
	}

	if cfg.CachePort == "" {
		return fmt.Errorf("redis port cannot be empty")
	}

	return nil
}

func NewRedisQueue(client *redis.Client, sender *email.Sender, logger *slog.Logger) *RedisQueue {
	return &RedisQueue{
		client: client,
		sender: sender,
		logger: logger,
	}
}

func (q *RedisQueue) EnqueueEmail(ctx context.Context, task EmailTask) error {
	if err := validateEmailTask(task); err != nil {
		return fmt.Errorf("invalid email task: %w", err)
	}

	taskJSON, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to serialize email task: %w", err)
	}

	if err := q.client.RPush(ctx, emailQueue, taskJSON).Err(); err != nil {
		return fmt.Errorf("failed to enqueue email task: %w", err)
	}

	q.logger.Info("Email task enqueued", "to", task.To, "subject", task.Subject)
	return nil
}

func validateEmailTask(task EmailTask) error {
	if task.To == "" {
		return fmt.Errorf("recipient email is required")
	}

	if task.Subject == "" {
		return fmt.Errorf("email subject is required")
	}

	if task.TemplateName == "" {
		return fmt.Errorf("email template name is required")
	}

	return nil
}

func (q *RedisQueue) StartWorker(ctx context.Context) {
	q.logger.Info("Starting email queue worker...")

	for {
		select {
		case <-ctx.Done():
			q.logger.Info("Email queue worker stopped")
			return
		default:
			if err := q.processNextTask(ctx); err != nil {
				q.logger.Error("Task processing error", "error", err)
				time.Sleep(queueCheckInterval)
			}
		}
	}
}

func (q *RedisQueue) processNextTask(ctx context.Context) error {
	result, err := q.client.BLPop(ctx, 0, emailQueue).Result()
	if err != nil {
		if err == redis.Nil || err == context.Canceled {
			return nil
		}
		return fmt.Errorf("queue retrieval error: %w", err)
	}

	if len(result) < 2 {
		return fmt.Errorf("invalid queue result")
	}

	var task EmailTask
	if err := json.Unmarshal([]byte(result[1]), &task); err != nil {
		return fmt.Errorf("task deserialization error: %w", err)
	}

	return q.sendEmailWithRetry(ctx, task)
}

func (q *RedisQueue) sendEmailWithRetry(ctx context.Context, task EmailTask) error {
	err := q.sender.SendEmail(task.To, task.Subject, task.TemplateName, task.Data)

	if err == nil {
		q.logger.Info("Email sent successfully", "to", task.To, "subject", task.Subject)
		return nil
	}

	if task.Retries < maxRetries {
		task.Retries++
		q.logger.Warn("Email send failed, requeueing",
			"to", task.To,
			"subject", task.Subject,
			"retries", task.Retries,
			"error", err,
		)

		time.Sleep(retryDelay)

		requeueErr := q.EnqueueEmail(ctx, task)
		if requeueErr != nil {
			return fmt.Errorf("failed to requeue email: %w (original error: %v)", requeueErr, err)
		}

		return nil
	}

	q.logger.Error("Email send failed after max retries",
		"to", task.To,
		"subject", task.Subject,
		"error", err,
	)

	return err
}
