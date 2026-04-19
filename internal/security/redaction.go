package security

import (
	"regexp"
	"strings"
)

// Redactor strips known secret patterns from log fields to prevent
// plaintext secrets from appearing in structured logs.
type Redactor struct {
	patterns []*regexp.Regexp
	mask     string
}

// RedactorConfig holds configuration for the log field redactor.
type RedactorConfig struct {
	// CustomPatterns adds additional regex patterns beyond the defaults.
	// Each pattern's first capture group (or the full match) is redacted.
	CustomPatterns []string
	// Mask is the replacement string (default: "[REDACTED]").
	Mask string
}

// defaultSecretPatterns matches common secret field patterns.
var defaultSecretPatterns = []string{
	// Match field values for known secret keys: "token": "value", "token":"value"
	`(?i)("?(?:token|access_token|refresh_token|api_key|secret_key|private_key|webhook_secret|password|auth_token|bearer_token)"?\s*[:=]\s*"?)[^",}\s]+`,
	// Match Bearer tokens in Authorization headers
	`(?i)(Bearer\s+)[A-Za-z0-9\-._~+/]+=*`,
	// Match long hex strings that look like secrets (>=32 chars after known prefixes)
	`(?i)((?:sk|pk|ghp|gho|ghs|ghu|github_pat|glpat|xox[bps])[-_])[A-Za-z0-9\-_]{20,}`,
	// Match private key content
	`(?s)(-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----).+?(-----END)`,
}

// NewRedactor creates a Redactor with default and optional custom patterns.
func NewRedactor(cfg RedactorConfig) (*Redactor, error) {
	mask := cfg.Mask
	if mask == "" {
		mask = "[REDACTED]"
	}

	allPatterns := make([]string, 0, len(defaultSecretPatterns)+len(cfg.CustomPatterns))
	allPatterns = append(allPatterns, defaultSecretPatterns...)
	allPatterns = append(allPatterns, cfg.CustomPatterns...)

	patterns := make([]*regexp.Regexp, 0, len(allPatterns))
	for _, p := range allPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, re)
	}

	return &Redactor{
		patterns: patterns,
		mask:     mask,
	}, nil
}

// Redact replaces all matched secret patterns in the input string with the mask.
func (r *Redactor) Redact(input string) string {
	result := input
	for _, re := range r.patterns {
		result = re.ReplaceAllStringFunc(result, func(match string) string {
			// If the pattern has capture groups, preserve the prefix (group 1)
			// and only mask the secret value.
			submatches := re.FindStringSubmatch(match)
			if len(submatches) >= 2 {
				prefix := submatches[1]
				return prefix + r.mask
			}
			return r.mask
		})
	}
	return result
}

// sensitiveKeys maps field names considered sensitive.
var sensitiveKeys = map[string]bool{
	"token":         true,
	"access_token":  true,
	"refresh_token": true,
	"api_key":       true,
	"secret_key":    true,
	"private_key":   true,
	"webhook_secret": true,
	"password":      true,
	"auth_token":    true,
	"bearer_token":  true,
	"authorization": true,
	"cookie":        true,
	"set_cookie":    true,
}

// RedactFields returns a new map with sensitive field values replaced by the mask.
// The original map is not modified.
func (r *Redactor) RedactFields(fields map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(fields))
	for k, v := range fields {
		lowerKey := strings.ToLower(k)
		if sensitiveKeys[lowerKey] {
			result[k] = r.mask
		} else {
			// Also redact string values that contain secret patterns
			if s, ok := v.(string); ok {
				result[k] = r.Redact(s)
			} else {
				result[k] = v
			}
		}
	}
	return result
}

// Mask returns the configured mask string (useful for testing).
func (r *Redactor) Mask() string {
	return r.mask
}
