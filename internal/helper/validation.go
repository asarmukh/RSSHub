package helper

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

func IsValidURL(feedURL string) error {
	u, err := url.ParseRequestURI(feedURL)
	if err != nil {
		return fmt.Errorf("invalid feed URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(feedURL)
	if err != nil {
		return fmt.Errorf("Couldnot reach URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Bad response status: %s", resp.Status)
	}

	return nil
}
