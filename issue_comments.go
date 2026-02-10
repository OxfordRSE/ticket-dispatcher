package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type ghComment struct {
	Body string `json:"body"`
}

func postIssueComment(issueNumber, msgId, comment string) error {
	exists, err := commentWithMessageIDExists(issueNumber, msgId)
	// only suppress posting if we get confirmation that Message-ID was found
	// better to post twice than silently fail
	if exists {
		return fmt.Errorf("Message-ID: %s already posted", msgId)
	}
	if err != nil {
		log.Printf("error from commentWithMessageIDExists: %v", err)
	}
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("missing environment variable GITHUB_TOKEN")
	}

	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/issues/%s/comments",
		githubProject, issueNumber,
	)
	payload := map[string]string{
		"body": fmt.Sprintf("Message-ID: %s\n", msgId) + comment,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ticket-dispatcher")

	client := &http.Client{
		Timeout: 20 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("github request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("github returned %s", resp.Status)
	}
	return nil
}

// commentWithMessageIDExists checks whether an issue already has a comment
// whose first line contains the given Message-ID (exact match or contains).
func commentWithMessageIDExists(issueNumber, messageID string) (bool, error) {
	token := os.Getenv("GITHUB_TOKEN")

	if token == "" {
		return false, fmt.Errorf("missing environment variable GITHUB_TOKEN")
	}

	needle := strings.TrimSpace("Message-ID: " + messageID)
	client := &http.Client{Timeout: 15 * time.Second}

	page := 1
	for {
		url := fmt.Sprintf(
			"https://api.github.com/repos/%s/issues/%s/comments?per_page=100&page=%d",
			githubProject, issueNumber, page,
		)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return false, err
		}
		req.Header.Set("Authorization", "token "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "ticket-dispatcher")

		resp, err := client.Do(req)
		if err != nil {
			return false, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("github list comments failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
		}

		var comments []ghComment
		if err := json.Unmarshal(body, &comments); err != nil {
			return false, fmt.Errorf("decode comments: %w", err)
		}

		// no more pages
		if len(comments) == 0 {
			return false, nil
		}

		for _, c := range comments {
			firstLine := c.Body
			if i := strings.IndexByte(c.Body, '\n'); i >= 0 {
				firstLine = c.Body[:i]
			}
			if strings.TrimSpace(firstLine) == needle {
				return true, nil
			}
		}
		page++
	}
}
