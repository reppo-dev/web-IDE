package llm

import (
	"regexp"
	"strings"
)

var (
	// skPatterns matches OpenAI, Anthropic and standard sk- style API keys.
	skPattern = regexp.MustCompile(`\b(sk-(?:ant-)?[a-zA-Z0-9_\-]{20,})\b`)

	// bearerPattern matches Authorization Bearer tokens.
	bearerPattern = regexp.MustCompile(`(?i)\b(Bearer\s+)([a-zA-Z0-9_\-\.]{20,})\b`)

	// ghPattern matches GitHub Personal Access Tokens.
	ghPattern = regexp.MustCompile(`\b(ghp_[a-zA-Z0-9]{36}|github_pat_[a-zA-Z0-9_]{82})\b`)

	// awsPattern matches AWS access key IDs.
	awsPattern = regexp.MustCompile(`\b(AKIA[0-9A-Z]{16})\b`)

	// googleKeyPattern matches Google API keys (AIzaSy...).
	googleKeyPattern = regexp.MustCompile(`\b(AIzaSy[a-zA-Z0-9_\-]{30,40})\b`)

	// nineRouterPattern matches NineRouter tokens.
	nineRouterPattern = regexp.MustCompile(`\b(nr-[a-zA-Z0-9_\-]{20,})\b`)

	// privateKeyPattern matches PEM formatted private keys.
	privateKeyPattern = regexp.MustCompile(`(?s)-----BEGIN[ A-Z0-9_-]*PRIVATE KEY-----.*?-----END[ A-Z0-9_-]*PRIVATE KEY-----`)

	// urlPasswordPattern matches passwords embedded in URLs (e.g. postgres://user:password@host).
	urlPasswordPattern = regexp.MustCompile(`([a-zA-Z]+://[^:\s@]+):([^@\s/]+)@`)
)

// RedactSecrets scans the input string and replaces sensitive tokens, API keys,
// private keys, and credential URLs with sanitized placeholders (PROJECT_VISION §25).
func RedactSecrets(text string) string {
	if text == "" {
		return ""
	}

	out := text

	// 1. Private keys
	out = privateKeyPattern.ReplaceAllString(out, "[REDACTED_PRIVATE_KEY]")

	// 2. OpenAI / Anthropic / Generic sk- keys
	out = skPattern.ReplaceAllString(out, "[REDACTED_API_KEY]")

	// 3. Bearer tokens
	out = bearerPattern.ReplaceAllString(out, "$1[REDACTED_TOKEN]")

	// 4. GitHub tokens
	out = ghPattern.ReplaceAllString(out, "[REDACTED_GITHUB_TOKEN]")

	// 5. AWS Access Keys
	out = awsPattern.ReplaceAllString(out, "[REDACTED_AWS_KEY]")

	// 6. Google API keys
	out = googleKeyPattern.ReplaceAllString(out, "[REDACTED_GOOGLE_KEY]")

	// 7. NineRouter keys
	out = nineRouterPattern.ReplaceAllString(out, "[REDACTED_NINEROUTER_KEY]")

	// 8. Passwords in URLs
	out = urlPasswordPattern.ReplaceAllString(out, "$1:***@")

	return out
}

// RedactSlice returns a new slice where each element has secrets redacted.
func RedactSlice(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	res := make([]string, len(items))
	for i, item := range items {
		res[i] = RedactSecrets(item)
	}
	return res
}

// MaskKey returns a masked string showing only the first few and last few characters,
// suitable for UI display without exposing the secret.
func MaskKey(key string) string {
	clean := strings.TrimSpace(key)
	if len(clean) <= 8 {
		return "********"
	}
	prefix := clean[:4]
	suffix := clean[len(clean)-4:]
	return prefix + "..." + suffix
}
