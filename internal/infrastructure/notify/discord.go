package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Discord sends alert notifications via Discord webhook.
type Discord struct {
	WebhookURL string
	Client     *http.Client
}

func NewDiscord(webhookURL string) *Discord {
	return &Discord{
		WebhookURL: webhookURL,
		Client:     &http.Client{Timeout: 10 * time.Second},
	}
}

type discordPayload struct {
	Embeds []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title     string         `json:"title"`
	Color     int            `json:"color"`
	Fields    []discordField `json:"fields"`
	Footer    discordFooter  `json:"footer"`
	Timestamp string         `json:"timestamp"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type discordFooter struct {
	Text string `json:"text"`
}

func (d *Discord) Send(level, nodeName string, usagePct float64, message string) error {
	if d.WebhookURL == "" {
		return fmt.Errorf("discord webhook URL not configured")
	}

	color := 3066993 // green
	emoji := "🟢"
	switch level {
	case "warning":
		color = 16776960 // yellow
		emoji = "🟡"
	case "critical":
		color = 15158332 // red
		emoji = "🔴"
	}

	payload := discordPayload{
		Embeds: []discordEmbed{{
			Title: fmt.Sprintf("%s %s: %s at %.1f%%", emoji, capitalize(level), nodeName, usagePct),
			Color: color,
			Fields: []discordField{
				{Name: "Node", Value: nodeName, Inline: true},
				{Name: "Usage", Value: fmt.Sprintf("%.1f%%", usagePct), Inline: true},
				{Name: "Status", Value: level, Inline: true},
				{Name: "Details", Value: message, Inline: false},
			},
			Footer:    discordFooter{Text: "Reaplet Storage Monitor"},
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := d.Client.Post(d.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discord webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("discord webhook returned %d", resp.StatusCode)
	}
	return nil
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}
