package notify

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const pushoverAPI = "https://api.pushover.net/1/messages.json"

// Pushover sends alert notifications via Pushover API.
type Pushover struct {
	AppToken string
	UserKey  string
	Client   *http.Client
}

func NewPushover(appToken, userKey string) *Pushover {
	return &Pushover{
		AppToken: appToken,
		UserKey:  userKey,
		Client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *Pushover) Send(level, nodeName string, usagePct float64, message string) error {
	if p.AppToken == "" || p.UserKey == "" {
		return fmt.Errorf("pushover credentials not configured")
	}

	priority := "0" // normal
	switch level {
	case "warning":
		priority = "0"
	case "critical":
		priority = "1" // high priority
	}

	// Emergency for > 95%
	if usagePct > 95 {
		priority = "2"
	}

	emoji := "🟢"
	switch level {
	case "warning":
		emoji = "🟡"
	case "critical":
		emoji = "🔴"
	}

	title := fmt.Sprintf("%s %s storage %s", emoji, nodeName, level)

	form := url.Values{
		"token":    {p.AppToken},
		"user":     {p.UserKey},
		"title":    {title},
		"message":  {message},
		"priority": {priority},
	}

	// Emergency priority requires retry/expire params
	if priority == "2" {
		form.Set("retry", "300")
		form.Set("expire", "3600")
	}

	resp, err := p.Client.PostForm(pushoverAPI, form)
	if err != nil {
		return fmt.Errorf("pushover request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("pushover returned %d", resp.StatusCode)
	}
	return nil
}
