package llm

import (
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "plain text without secrets",
			input:    "Hello world, package main func main() {}",
			expected: "Hello world, package main func main() {}",
		},
		{
			name:     "OpenAI sk- key",
			input:    "Found key sk-1234567890abcdef1234567890abcdef in response",
			expected: "Found key [REDACTED_API_KEY] in response",
		},
		{
			name:     "Anthropic sk-ant- key",
			input:    "Using token sk-ant-api03-abcdef1234567890abcdef1234567890",
			expected: "Using token [REDACTED_API_KEY]",
		},
		{
			name:     "Authorization Bearer header",
			input:    "Authorization: Bearer mySecretToken1234567890abcdef",
			expected: "Authorization: Bearer [REDACTED_TOKEN]",
		},
		{
			name:     "GitHub PAT",
			input:    "git clone with ghp_1234567890abcdef1234567890abcdef1234",
			expected: "git clone with [REDACTED_GITHUB_TOKEN]",
		},
		{
			name:     "AWS Access Key",
			input:    "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
			expected: "AWS_ACCESS_KEY_ID=[REDACTED_AWS_KEY]",
		},
		{
			name:     "Google API Key",
			input:    "key=AIzaSyA1234567890abcdef1234567890abcde",
			expected: "key=[REDACTED_GOOGLE_KEY]",
		},
		{
			name:     "NineRouter Token",
			input:    "token: nr-1234567890abcdef1234567890",
			expected: "token: [REDACTED_NINEROUTER_KEY]",
		},
		{
			name:     "Private Key block",
			input:    "Cert:\n-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA0\n-----END RSA PRIVATE KEY-----\nEnd",
			expected: "Cert:\n[REDACTED_PRIVATE_KEY]\nEnd",
		},
		{
			name:     "Database URL with password",
			input:    "connect to postgres://admin:superSecret123@localhost:5432/mydb",
			expected: "connect to postgres://admin:***@localhost:5432/mydb",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactSecrets(tc.input)
			if got != tc.expected {
				t.Errorf("RedactSecrets(%q)\ngot  %q\nwant %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestMaskKey(t *testing.T) {
	if got := MaskKey("short"); got != "********" {
		t.Errorf("MaskKey(short) = %q, want ********", got)
	}
	key := "sk-1234567890abcdef"
	got := MaskKey(key)
	if !strings.HasPrefix(got, "sk-1") || !strings.HasSuffix(got, "cdef") {
		t.Errorf("MaskKey(%q) = %q, unexpected masking", key, got)
	}
}
