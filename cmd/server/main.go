package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sarthakyeole/redis-go-mailing-bulk/api"
	"github.com/sarthakyeole/redis-go-mailing-bulk/internal/config"
	templates "github.com/sarthakyeole/redis-go-mailing-bulk/internal/emailTemplate"
	queue "github.com/sarthakyeole/redis-go-mailing-bulk/internal/redisQueue"
	email "github.com/sarthakyeole/redis-go-mailing-bulk/internal/senderSide"
)

func main() {
	cfg := config.LoadConfiguration()

	tmpl, err := templates.New()
	if err != nil {
		log.Fatalf("Error initializing templates: %v", err)
	}

	redisClient, err := queue.NewRedisClient(cfg)
	if err != nil {
		log.Fatalf("Error connecting to Redis: %v", err)
	}
	defer redisClient.Close()

	emailService := email.NewSender(cfg, tmpl)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	redisQueue := queue.NewRedisQueue(redisClient, emailService, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go redisQueue.StartWorker(ctx)

	router := gin.Default()
	api.RegisterHandlers(router, redisQueue)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.ServerPort),
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting server: %v", err)
		}
	}()

	log.Printf("Server started on port %s", cfg.ServerPort)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Error shutting down server: %v", err)
	}

	log.Println("Server shut down successfully")
}
