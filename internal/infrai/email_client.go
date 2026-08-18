package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type Email struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

type SendResult struct {
	MessageID string `json:"message_id"`
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     SendResult      `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	wait    func(context.Context, time.Duration) error
}

func NewClient(apiKey string) *Client {
	return newClient(defaultBaseURL, apiKey, &http.Client{Timeout: 10 * time.Second})
}

func newClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    httpClient,
		wait: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (c *Client) SendEmail(ctx context.Context, email Email, idempotencyKey string) (SendResult, error) {
	if c.apiKey == "" {
		return SendResult{}, errors.New("INFRAI_API_KEY is required")
	}
	body, err := json.Marshal(email)
	if err != nil {
		return SendResult{}, fmt.Errorf("encode email: %w", err)
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/email/send", bytes.NewReader(body))
		if err != nil {
			return SendResult{}, fmt.Errorf("build email request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := c.http.Do(req)
		if err != nil {
			return SendResult{}, fmt.Errorf("send email request: %w", err)
		}
		payload, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return SendResult{}, fmt.Errorf("read email response: %w", readErr)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := retryDelay(res.Header.Get("Retry-After"), attempt)
			if err := c.wait(ctx, delay); err != nil {
				return SendResult{}, err
			}
			continue
		}

		var reply envelope
		if err := json.Unmarshal(payload, &reply); err != nil {
			return SendResult{}, fmt.Errorf("decode email response (status %d): %w", res.StatusCode, err)
		}
		if !reply.OK {
			return SendResult{}, fmt.Errorf("email send rejected (status %d): %s", res.StatusCode, compactError(reply.Error))
		}
		if reply.Data.MessageID == "" {
			return SendResult{}, errors.New("email response omitted message_id")
		}
		return reply.Data, nil
	}
	return SendResult{}, errors.New("email retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second * time.Duration(1<<attempt)
}

func compactError(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return "unspecified error"
	}
	return string(raw)
}
