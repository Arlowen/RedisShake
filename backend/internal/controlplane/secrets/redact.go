package secrets

import "regexp"

var redactionPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{
		pattern:     regexp.MustCompile(`(?i)(redis(?:s)?://[^:/@\s]+:)[^@\s]+(@)`),
		replacement: `${1}******${2}`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)("(?:password|token|secret)"\s*:\s*")[^"]*(")`),
		replacement: `${1}******${2}`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)((?:password|token|secret)\s*[=:]\s*)[^\s,;]+`),
		replacement: `${1}******`,
	},
}

func Redact(value string) string {
	for _, item := range redactionPatterns {
		value = item.pattern.ReplaceAllString(value, item.replacement)
	}
	return value
}
