package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type GroqDiagnoser struct {
	APIKey  string
	Model   string
	BaseURL string
	HTTP    *http.Client
}

type groqReq struct {
	Model    string        `json:"model"`
	Messages []groqMessage `json:"messages"`
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (g *GroqDiagnoser) Diagnose(ctx context.Context, jobType, errorClass, errorRaw string, payloadKeys []string, retries int, durationMs int64) (string, error) {
	if g == nil || g.APIKey == "" {
		return "", fmt.Errorf("groq disabled")
	}
	base := strings.TrimRight(g.BaseURL, "/")
	url := base + "/chat/completions"
	user := fmt.Sprintf(
		"job_type=%s error_class=%s error_raw=%s payload_keys=%v retries=%d duration_ms=%d",
		jobType, errorClass, errorRaw, payloadKeys, retries, durationMs,
	)
	body, _ := json.Marshal(groqReq{
		Model: g.Model,
		Messages: []groqMessage{
			{Role: "system", Content: "You diagnose background job failures. Be short. Suggest retry vs poison vs payload bug."},
			{Role: "user", Content: user},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+g.APIKey)
	req.Header.Set("Content-Type", "application/json")
	client := g.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var parsed groqResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", fmt.Errorf("groq: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("groq: empty choices (%s)", string(raw))
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}
