package email

import (
	"bytes"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/sarthakyeole/redis-go-mailing-bulk/internal/config"
	templates "github.com/sarthakyeole/redis-go-mailing-bulk/internal/emailTemplate"
)

type Sender struct {
	config    *config.ApplicationConfig
	templates *templates.Manager
}

func NewSender(cfg *config.ApplicationConfig, tmpl *templates.Manager) *Sender {
	return &Sender{
		config:    cfg,
		templates: tmpl,
	}
}

func (s *Sender) SendEmail(to, subject, templateName string, data map[string]interface{}) error {
	// Validate inputs
	if to == "" {
		return fmt.Errorf("recipient email address cannot be empty")
	}
	if subject == "" {
		return fmt.Errorf("email subject cannot be empty")
	}
	if templateName == "" {
		return fmt.Errorf("email template name cannot be empty")
	}

	// Validate SMTP configuration
	if err := s.validateSMTPConfig(); err != nil {
		return fmt.Errorf("invalid SMTP configuration: %w", err)
	}

	// Render email template
	body, err := s.templates.RenderWithSafeURLs(templateName, data)
	if err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}

	// Prepare email message
	var message bytes.Buffer
	message.WriteString(fmt.Sprintf("From: %s <%s>\r\n", s.config.EmailSenderDisplayName, s.config.EmailSenderAddress))
	message.WriteString(fmt.Sprintf("To: %s\r\n", to))
	message.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	message.WriteString(body)

	// Prepare SMTP connection
	addr := fmt.Sprintf("%s:%d", s.config.EmailSMTPServer, s.config.EmailSMTPServerPort)

	// Create authentication
	auth := smtp.PlainAuth(
		"",
		s.config.EmailSMTPUsername,
		s.config.EmailSMTPPassword,
		s.config.EmailSMTPServer,
	)

	// Send email using standard library method with TLS
	return smtp.SendMail(
		addr,
		auth,
		s.config.EmailSenderAddress,
		[]string{to},
		message.Bytes(),
	)
}

func (s *Sender) validateSMTPConfig() error {
	if strings.TrimSpace(s.config.EmailSMTPServer) == "" {
		return fmt.Errorf("SMTP server is not configured")
	}
	if s.config.EmailSMTPServerPort <= 0 {
		return fmt.Errorf("invalid SMTP server port")
	}
	if strings.TrimSpace(s.config.EmailSenderAddress) == "" {
		return fmt.Errorf("sender email address is not configured")
	}
	if strings.TrimSpace(s.config.EmailSMTPUsername) == "" {
		return fmt.Errorf("SMTP username is not configured")
	}
	if strings.TrimSpace(s.config.EmailSMTPPassword) == "" {
		return fmt.Errorf("SMTP password is not configured")
	}
	return nil
}

func (s *Sender) SendTemplatedEmail(to, subject, templateName string, data map[string]interface{}) error {
	return s.SendEmail(to, subject, templateName, data)
}
