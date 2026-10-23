package core

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"privacyos/backend/internal/config"
	"privacyos/backend/internal/store"
)

// Notifier delivers outbound notifications through the configured channels.
type Notifier struct {
	cfg *config.Config
	st  *store.Store
}

// SendResult reports the outcome per channel.
type SendResult struct {
	Channel string `json:"channel"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

const keepSecret = "__keep__"

// Send delivers a message to every enabled channel.
func (n *Notifier) Send(ctx context.Context, title, message string) []SendResult {
	var s NotificationSettings
	if _, err := n.st.Setting("notifications", &s); err != nil {
		return []SendResult{{Channel: "config", Error: err.Error()}}
	}
	var results []SendResult
	if s.EmailEnabled && s.SMTPHost != "" && s.SMTPTo != "" {
		err := n.sendEmail(s, title, message)
		results = append(results, SendResult{Channel: "email", OK: err == nil, Error: errText(err)})
	}
	if s.TelegramEnable && s.TelegramToken != "" && s.TelegramChatID != "" {
		err := n.sendTelegram(ctx, s, title, message)
		results = append(results, SendResult{Channel: "telegram", OK: err == nil, Error: errText(err)})
	}
	if s.BarkEnabled && s.BarkURL != "" {
		err := n.sendBark(ctx, s, title, message)
		results = append(results, SendResult{Channel: "bark", OK: err == nil, Error: errText(err)})
	}
	if s.ServerChanKey != "" {
		err := n.sendServerChan(ctx, s, title, message)
		results = append(results, SendResult{Channel: "serverchan", OK: err == nil, Error: errText(err)})
	}
	if len(results) == 0 {
		results = append(results, SendResult{Channel: "none", OK: false, Error: "未启用任何通知渠道"})
	}
	return results
}

func (n *Notifier) sendEmail(s NotificationSettings, title, message string) error {
	from := s.SMTPFrom
	if from == "" {
		from = s.SMTPUser
	}
	header := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\nDate: %s\r\n\r\n%s",
		from, s.SMTPTo, title, time.Now().Format(time.RFC1123Z), message)
	auth := smtp.PlainAuth("", s.SMTPUser, s.SMTPPassword, s.SMTPHost)
	if s.SMTPPort == 465 {
		conn, err := tls.Dial("tcp", net.JoinHostPort(s.SMTPHost, "465"),
			&tls.Config{ServerName: s.SMTPHost, MinVersion: tls.VersionTLS12})
		if err != nil {
			return err
		}
		defer conn.Close()
		c, err := smtp.NewClient(conn, s.SMTPHost)
		if err != nil {
			return err
		}
		defer c.Close()
		if err := c.Auth(auth); err != nil {
			return err
		}
		if err := c.Mail(from); err != nil {
			return err
		}
		if err := c.Rcpt(s.SMTPTo); err != nil {
			return err
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(header)); err != nil {
			return err
		}
		return w.Close()
	}
	addr := net.JoinHostPort(s.SMTPHost, fmt.Sprint(s.SMTPPort))
	return smtp.SendMail(addr, auth, from, strings.Split(s.SMTPTo, ","), []byte(header))
}

func (n *Notifier) sendTelegram(ctx context.Context, s NotificationSettings, title, message string) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.TelegramToken)
	form := url.Values{}
	form.Set("chat_id", s.TelegramChatID)
	form.Set("text", title+"\n\n"+message)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return n.doJSON(req)
}

func (n *Notifier) sendBark(ctx context.Context, s NotificationSettings, title, message string) error {
	base := strings.TrimRight(s.BarkURL, "/")
	u := fmt.Sprintf("%s/%s/%s", base, url.PathEscape(title), url.PathEscape(message))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return n.doJSON(req)
}

func (n *Notifier) sendServerChan(ctx context.Context, s NotificationSettings, title, message string) error {
	endpoint := fmt.Sprintf("https://sctapi.ftqq.com/%s.send", s.ServerChanKey)
	form := url.Values{}
	form.Set("title", title)
	form.Set("desp", message)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return n.doJSON(req)
}

func (n *Notifier) doJSON(req *http.Request) error {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
