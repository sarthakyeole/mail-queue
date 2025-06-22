package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	queue "github.com/sarthakyeole/redis-go-mailing-bulk/internal/redisQueue"
)

var validate = validator.New()

type ErrorResponse struct {
	Error     string            `json:"error"`
	Details   map[string]string `json:"details,omitempty"`
	RequestID string            `json:"requestId,omitempty"`
}

type SendEmailRequest struct {
	To           string                 `json:"to" binding:"required,email" validate:"required,email"`
	Subject      string                 `json:"subject" binding:"required" validate:"required,min=1,max=200"`
	TemplateName string                 `json:"templateName" binding:"required" validate:"required,min=1,max=50"`
	Data         map[string]interface{} `json:"data" binding:"required" validate:"required"`
}

func RegisterHandlers(router *gin.Engine, redisQueue *queue.RedisQueue) {
	router.Use(corsMiddleware())

	router.Use(globalErrorHandler())

	router.GET("/health", healthCheck)

	api := router.Group("/api")
	{
		api.POST("/send", sendEmailHandler(redisQueue))
		api.POST("/bulk-send", bulkEmailHandler(redisQueue))
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

func globalErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error: "internal server error",
					Details: map[string]string{
						"message": "an unexpected error occurred",
					},
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"timestamp": gin.H{
			"server": gin.H{
				"time":     time.Now().UTC(),
				"timezone": "UTC",
			},
		},
	})
}

func validateRequest(req interface{}) error {
	if err := validate.Struct(req); err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			return errors.New("invalid request structure")
		}

		var errorDetails = make(map[string]string)
		for _, e := range err.(validator.ValidationErrors) {
			switch e.Tag() {
			case "required":
				errorDetails[e.Field()] = "this field is required"
			case "email":
				errorDetails[e.Field()] = "invalid email format"
			case "min":
				errorDetails[e.Field()] = "value is too short"
			case "max":
				errorDetails[e.Field()] = "value is too long"
			default:
				errorDetails[e.Field()] = "validation failed"
			}
		}

		return &ValidationError{
			Errors: errorDetails,
		}
	}
	return nil
}

type ValidationError struct {
	Errors map[string]string
}

func (e *ValidationError) Error() string {
	var errStrings []string
	for field, msg := range e.Errors {
		errStrings = append(errStrings, field+": "+msg)
	}
	return strings.Join(errStrings, "; ")
}

func sendEmailHandler(redisQueue *queue.RedisQueue) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SendEmailRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: "invalid request",
				Details: map[string]string{
					"message": err.Error(),
				},
			})
			return
		}

		if err := validateRequest(&req); err != nil {
			switch e := err.(type) {
			case *ValidationError:
				c.JSON(http.StatusBadRequest, ErrorResponse{
					Error:   "validation failed",
					Details: e.Errors,
				})
			default:
				c.JSON(http.StatusBadRequest, ErrorResponse{
					Error: err.Error(),
				})
			}
			return
		}

		sanitizedData := sanitizeTemplateData(req.Data)

		task := queue.EmailTask{
			To:           strings.TrimSpace(req.To),
			Subject:      strings.TrimSpace(req.Subject),
			TemplateName: strings.TrimSpace(req.TemplateName),
			Data:         sanitizedData,
		}

		if err := redisQueue.EnqueueEmail(c.Request.Context(), task); err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error: "failed to queue email",
				Details: map[string]string{
					"reason": err.Error(),
				},
			})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"message": "email was successfully added to the queue",
			"details": gin.H{
				"recipient": task.To,
				"subject":   task.Subject,
			},
		})
	}
}

func bulkEmailHandler(redisQueue *queue.RedisQueue) gin.HandlerFunc {
	type BulkEmailRequest struct {
		Emails []SendEmailRequest `json:"emails" binding:"required,min=1,max=50" validate:"required,min=1,max=50"`
	}

	return func(c *gin.Context) {
		var req BulkEmailRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "invalid bulk email request",
				Details: map[string]string{"message": err.Error()},
			})
			return
		}

		var failedEmails []string
		var successEmails []string

		for _, emailReq := range req.Emails {
			if err := validateRequest(&emailReq); err != nil {
				failedEmails = append(failedEmails, emailReq.To)
				continue
			}

			task := queue.EmailTask{
				To:           strings.TrimSpace(emailReq.To),
				Subject:      strings.TrimSpace(emailReq.Subject),
				TemplateName: strings.TrimSpace(emailReq.TemplateName),
				Data:         sanitizeTemplateData(emailReq.Data),
			}

			if err := redisQueue.EnqueueEmail(c.Request.Context(), task); err != nil {
				failedEmails = append(failedEmails, task.To)
			} else {
				successEmails = append(successEmails, task.To)
			}
		}

		if len(failedEmails) > 0 {
			c.JSON(http.StatusMultiStatus, gin.H{
				"message":       "partial success in queueing emails",
				"successCount":  len(successEmails),
				"failedCount":   len(failedEmails),
				"successEmails": successEmails,
				"failedEmails":  failedEmails,
			})
		} else {
			c.JSON(http.StatusAccepted, gin.H{
				"message":       "all emails successfully queued",
				"successCount":  len(successEmails),
				"successEmails": successEmails,
			})
		}
	}
}

func sanitizeTemplateData(data map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})
	for k, v := range data {
		switch val := v.(type) {
		case string:
			sanitized[k] = strings.TrimSpace(val)
		case int, int64, float64, bool:
			sanitized[k] = val
		default:
			sanitized[k] = fmt.Sprintf("%v", val)
		}
	}
	return sanitized
}
