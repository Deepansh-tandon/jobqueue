package jobs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var (
	uuidRE  = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	digitRE = regexp.MustCompile(`[0-9]+`)
)

func Fingerprint(jobType, errRaw string, payloadJSON string) (fp, errorClass string, keys []string) {
	keys = PayloadKeys(payloadJSON)
	errorClass = ErrorClass(errRaw)
	sort.Strings(keys)
	joined := jobType + "|" + errorClass + "|" + strings.Join(keys, ",")
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:]), errorClass, keys
}

func ErrorClass(errRaw string) string {
	line := strings.TrimSpace(errRaw)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	if line == "" {
		return "unknown"
	}
	line = strings.ToLower(line)
	line = uuidRE.ReplaceAllString(line, "")
	line = digitRE.ReplaceAllString(line, "")
	var b strings.Builder
	prevSpace := false
	for _, r := range line {
		if unicode.IsLetter(r) || r == '_' || r == ':' || r == '.' || r == '-' {
			b.WriteRune(r)
			prevSpace = false
			continue
		}
		if unicode.IsSpace(r) || r == ',' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "unknown"
	}
	return out
}

func PayloadKeys(payloadJSON string) []string {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(payloadJSON), &m); err != nil || m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func RequestHash(projectID, jobType string, payload map[string]interface{}) (string, error) {
	canonical := struct {
		ProjectID string                 `json:"project_id"`
		Type      string                 `json:"type"`
		Payload   map[string]interface{} `json:"payload"`
	}{projectID, jobType, payload}
	b, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
