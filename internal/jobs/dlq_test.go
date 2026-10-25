package jobs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFingerprintGroupsSamePoison(t *testing.T) {
	n := 20
	var first string
	for i := 0; i < n; i++ {
		fp, _, _ := Fingerprint("always_fail", "always_fail: poison job", `{"k":true}`)
		if i == 0 {
			first = fp
		} else {
			assert.Equal(t, first, fp)
		}
	}
}
