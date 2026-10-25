package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroqDiagnoserOnce(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer test-key")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "poison payload, do not retry"}},
			},
		})
	}))
	defer srv.Close()

	d := &GroqDiagnoser{APIKey: "test-key", Model: "llama-3.1-8b-instant", BaseURL: srv.URL, HTTP: srv.Client()}
	text, err := d.Diagnose(context.Background(), "always_fail", "always_fail poison job", "raw", []string{"x"}, 4, 10)
	require.NoError(t, err)
	assert.Contains(t, text, "poison")
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestGroqDiagnoserDisabled(t *testing.T) {
	d := &GroqDiagnoser{}
	_, err := d.Diagnose(context.Background(), "t", "e", "r", nil, 0, 0)
	assert.Error(t, err)
}
