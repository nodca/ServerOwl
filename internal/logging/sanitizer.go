package logging

import (
  "regexp"
  "strings"
)

// Sanitizer handles sensitive data masking
type Sanitizer struct {
  patterns    []*regexp.Regexp
  keyPatterns []string
  maskChar    string
  maskLength  int
}

// NewSanitizer creates a new sanitizer with default patterns
func NewSanitizer() *Sanitizer {
  s := &Sanitizer{
    maskChar:   "*",
    maskLength: 8,
    keyPatterns: []string{
      "password",
      "passwd",
      "secret",
      "token",
      "api_key",
      "apikey",
      "auth",
      "credential",
      "private",
      "key",
    },
  }

  // Compile regex patterns for common sensitive data
  patterns := []string{
    // API keys and tokens (generic patterns)
    `(?i)(api[_-]?key|token|secret)[=:]\s*["']?([a-zA-Z0-9_\-]{16,})["']?`,
    // Passwords in URLs
    `://[^:]+:([^@]+)@`,
    // Bearer tokens
    `(?i)bearer\s+([a-zA-Z0-9_\-\.]+)`,
    // Basic auth
    `(?i)basic\s+([a-zA-Z0-9+/=]+)`,
    // PostgreSQL DSN password
    `(?i)password=([^\s&]+)`,
    // Redis password
    `(?i)redis://:[^@]+@`,
    // Private keys
    `-----BEGIN\s+(?:RSA\s+)?PRIVATE\s+KEY-----`,
    // Credit card numbers (basic pattern)
    `\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`,
    // Email addresses (partial masking)
    `([a-zA-Z0-9._%+-]+)@([a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`,
    // IP addresses with ports (for internal IPs)
    `\b(10\.\d{1,3}\.\d{1,3}\.\d{1,3}|172\.(1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3}):\d+\b`,
  }

  for _, p := range patterns {
    if re, err := regexp.Compile(p); err == nil {
      s.patterns = append(s.patterns, re)
    }
  }

  return s
}

// SanitizeValue sanitizes a value based on its key name
func (s *Sanitizer) SanitizeValue(key, value string) string {
  keyLower := strings.ToLower(key)

  // Check if key matches sensitive patterns
  for _, pattern := range s.keyPatterns {
    if strings.Contains(keyLower, pattern) {
      return s.mask(value)
    }
  }

  // Apply regex patterns
  return s.SanitizeString(value)
}

// SanitizeString sanitizes sensitive data in a string
func (s *Sanitizer) SanitizeString(value string) string {
  result := value

  for _, re := range s.patterns {
    result = re.ReplaceAllStringFunc(result, func(match string) string {
      // Preserve structure but mask sensitive parts
      return s.maskMatch(match, re)
    })
  }

  return result
}

// mask replaces a value with mask characters
func (s *Sanitizer) mask(value string) string {
  if len(value) == 0 {
    return value
  }

  // Show first and last character for context
  if len(value) <= 4 {
    return strings.Repeat(s.maskChar, s.maskLength)
  }

  return string(value[0]) + strings.Repeat(s.maskChar, s.maskLength) + string(value[len(value)-1])
}

// maskMatch masks sensitive parts of a regex match
func (s *Sanitizer) maskMatch(match string, re *regexp.Regexp) string {
  submatches := re.FindStringSubmatch(match)
  if len(submatches) < 2 {
    return strings.Repeat(s.maskChar, s.maskLength)
  }

  // Replace the captured group with masked version
  result := match
  for i := 1; i < len(submatches); i++ {
    if submatches[i] != "" {
      result = strings.Replace(result, submatches[i], s.mask(submatches[i]), 1)
    }
  }

  return result
}

// AddKeyPattern adds a new sensitive key pattern
func (s *Sanitizer) AddKeyPattern(pattern string) {
  s.keyPatterns = append(s.keyPatterns, strings.ToLower(pattern))
}

// AddRegexPattern adds a new regex pattern for sanitization
func (s *Sanitizer) AddRegexPattern(pattern string) error {
  re, err := regexp.Compile(pattern)
  if err != nil {
    return err
  }
  s.patterns = append(s.patterns, re)
  return nil
}

// SanitizeMap sanitizes all string values in a map
func (s *Sanitizer) SanitizeMap(m map[string]any) map[string]any {
  result := make(map[string]any, len(m))
  for k, v := range m {
    switch val := v.(type) {
    case string:
      result[k] = s.SanitizeValue(k, val)
    case map[string]any:
      result[k] = s.SanitizeMap(val)
    default:
      result[k] = v
    }
  }
  return result
}
