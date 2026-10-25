package test

import (
	"os"
	"testing"
)

func TestAPIPlaceholdersRequireInfra(t *testing.T) {
	if os.Getenv("POSTGRES_DSN") == "" || os.Getenv("REDIS_URL") == "" {
		t.Skip("set POSTGRES_DSN and REDIS_URL for HTTP integration tests")
	}
}
