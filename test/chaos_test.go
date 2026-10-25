package test

import (
	"os"
	"testing"
)

func TestChaosRequiresInfra(t *testing.T) {
	if os.Getenv("CHAOS") != "1" {
		t.Skip("set CHAOS=1 with Redis+Postgres to run kill-worker / flood tests")
	}
}
