package notifier

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

type EmailChannel struct {
	smtpHost string
	smtpPort int
	from     string
	to       []string
	username string
	password string
}

func NewEmailChannel(smtpHost string, smtpPort int, from string, to []string, username, password string) *EmailChannel {
	return &EmailChannel{
		smtpHost: smtpHost,
		smtpPort: smtpPort,
		from:     from,
		to:       to,
		username: username,
		password: password,
	}
}

func (e *EmailChannel) Name() string { return "email" }

func (e *EmailChannel) Send(ctx context.Context, event *AlertEvent) error {
	subject := fmt.Sprintf("[dbinspect] %s Alert: %s (score: %d)", event.RiskLevel, event.CheckType, event.RiskScore)

	body := fmt.Sprintf(`<html><body>
<h2 style="color: %s;">dbinspect Alert: %s</h2>
<table>
<tr><td><b>Target:</b></td><td>%s</td></tr>
<tr><td><b>Check:</b></td><td>%s</td></tr>
<tr><td><b>Risk Score:</b></td><td>%d/100</td></tr>
<tr><td><b>Risk Level:</b></td><td>%s</td></tr>
<tr><td><b>Summary:</b></td><td>%s</td></tr>
<tr><td><b>Run ID:</b></td><td>%s</td></tr>
<tr><td><b>Time:</b></td><td>%s</td></tr>
</table>
</body></html>`,
		riskColor(event.RiskScore), event.RiskLevel,
		event.TargetName, event.CheckType,
		event.RiskScore, event.RiskLevel,
		event.Summary, event.RunID,
		event.OccurredAt.Format("2006-01-02 15:04:05"))

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		e.from, strings.Join(e.to, ","), subject, body)

	addr := fmt.Sprintf("%s:%d", e.smtpHost, e.smtpPort)

	var auth smtp.Auth
	if e.username != "" {
		auth = smtp.PlainAuth("", e.username, e.password, e.smtpHost)
	}

	if err := smtp.SendMail(addr, auth, e.from, e.to, []byte(msg)); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func riskColor(score int) string {
	switch {
	case score >= 80:
		return "#ff0000"
	case score >= 60:
		return "#ff8c00"
	case score >= 40:
		return "#ffd700"
	default:
		return "#333333"
	}
}
