# Email Queue Implementation

## Overview

This is a scalable email queue implementation in Go, utilizing Redis for queue management and providing a flexible email sending system with retry mechanisms and template support.

## Features

- Redis-backed Queue: Leverages Redis for reliable email task queuing
- Retry Mechanism: Automatic retries for failed email sends
- Template Engine: Supports dynamic email templating
- Configurable: Highly configurable through environment variables
- SMTP Email Sending: Supports configurable SMTP email sending
- Bulk Email Sending: Support for sending multiple emails in a single request

## API Endpoints

### Health Check

- Endpoint: `GET /health`
- Description: Checks the health of the application
- Response:
  ```json
  {
    "status": "ok",
    "timestamp": {
      "server": {
        "time": "2024-03-27T10:15:30Z",
        "timezone": "UTC"
      }
    }
  }
  ```

### Single Email Send

- Endpoint: `POST /api/send`
- Description: Enqueue a single email to be sent
- Request Body:
  ```json
  {
    "to": "recipient@gmail.com",
    "subject": "Mail regarding license update",
    "templateName": "license_update",
    "data": {
      "username": "License creator"
    }
  }
  ```
- Successful Response:
  ```json
  {
    "message": "email was successfully added to the queue",
    "details": {
      "recipient": "recipient@gmail.com",
      "subject": "Mail regarding license update"
    }
  }
  ```
- Error Responses:
  - `400 Bad Request`: Validation errors
  - `500 Internal Server Error`: Queueing failure

### Bulk Email Send

- Endpoint: `POST /api/bulk-send`
- Description: Enqueue multiple emails in a single request
- Request Body:
  ```json
  {
    "emails": [
      {
        "to": "user1@gmail.com",
        "subject": "Welcome User 1",
        "templateName": "license_update",
        "data": {
          "username": "License creator"
        }
      },
      {
        "to": "user2@gmail.com",
        "subject": "Welcome User 2",
        "templateName": "license_update",
        "data": {
          "username": "License creater"
        }
      }
    ]
  }
  ```
- Request Validation:

  - Minimum 1 email
  - Maximum 50 emails per request

- Successful Response (All emails queued):

  ```json
  {
    "message": "all emails successfully queued",
    "successCount": 2,
    "successEmails": ["user1@gmail.com", "user2@gmail.com"]
  }
  ```

- Partial Success Response:
  ```json
  {
    "message": "partial success in queueing emails",
    "successCount": 1,
    "failedCount": 1,
    "successEmails": ["user1@gmail.com"],
    "failedEmails": ["user2@gmail.com"]
  }
  ```

## Configuration

### Environment Variables

| Variable               | Description          | Default               |
| ---------------------- | -------------------- | --------------------- |
| `SERVER_PORT`          | HTTP server port     | `8080`                |
| `CACHE_HOST`           | Redis host           | `localhost`           |
| `CACHE_PORT`           | Redis port           | `6379`                |
| `CACHE_PASSWORD`       | Redis password       | `""`                  |
| `CACHE_DB_INDEX`       | Redis database index | `0`                   |
| `EMAIL_SMTP_SERVER`    | SMTP server address  | `smtp.gmail.com`      |
| `EMAIL_SMTP_PORT`      | SMTP server port     | `587`                 |
| `EMAIL_SMTP_USERNAME`  | SMTP username        | `recipient@gmail.com` |
| `EMAIL_SMTP_PASSWORD`  | SMTP password        | -                     |
| `EMAIL_SENDER_ADDRESS` | Sender email address | `recipient@gmail.com` |
| `EMAIL_SENDER_NAME`    | Sender display name  | `Sarthak`             |

## Email Queue Workflow

1. Create an `EmailTask` with recipient, subject, template, and data
2. Enqueue the email task to Redis
3. Background worker picks up the task
4. Attempts to send email with configurable retries
5. Logs success or failure

### Retry Strategy

- Maximum retries: 3
- Retry delay: 5 seconds between attempts
- Queue check interval: 1 second

## Installation

```bash
# Clone the repository
git clone https://github.com/sarthakyeole/redis-go-mailing-bulk.git

# Set required environment variables

# Run the application
go run ./cmd/server/main.go
```

## Dependencies

- Go 1.20+
- Redis
- gin-gonic/gin
- go-redis/redis
- html/template standard library

## Performance Considerations

- Uses connection pooling for Redis
- Non-blocking queue processing
- Configurable pool sizes and timeouts
- Structured logging for performance tracking

## Security

- URL and HTML escaping in templates
- Configurable SMTP authentication
- Environment-based configuration management
- Input validation for email tasks

## Authors

Sarthak Yeole
