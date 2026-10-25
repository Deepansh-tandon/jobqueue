package jobs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFingerprintStableAndIgnoresValues(t *testing.T) {
	fp1, class1, keys1 := Fingerprint("always_fail", "always_fail: poison job", `{"a":1,"b":"x"}`)
	fp2, class2, keys2 := Fingerprint("always_fail", "always_fail: poison job", `{"b":"other","a":999}`)
	assert.Equal(t, fp1, fp2)
	assert.Equal(t, class1, class2)
	assert.Equal(t, keys1, keys2)
}

func TestFingerprintDiffersOnKeys(t *testing.T) {
	fp1, _, _ := Fingerprint("always_fail", "always_fail: poison job", `{"a":1}`)
	fp2, _, _ := Fingerprint("always_fail", "always_fail: poison job", `{"a":1,"c":true}`)
	assert.NotEqual(t, fp1, fp2)
}

func TestErrorClassStripsUUIDsAndDigits(t *testing.T) {
	a := ErrorClass("Job 550e8400-e29b-41d4-a716-446655440000 failed after 3 retries")
	b := ErrorClass("Job 11111111-1111-1111-1111-111111111111 failed after 9 retries")
	assert.Equal(t, a, b)
}

func TestRequestHash(t *testing.T) {
	h1, err := RequestHash("p", "echo", map[string]interface{}{"z": 1, "a": 2})
	require.NoError(t, err)
	h2, err := RequestHash("p", "echo", map[string]interface{}{"a": 2, "z": 1})
	require.NoError(t, err)
	assert.Equal(t, h1, h2)
}
