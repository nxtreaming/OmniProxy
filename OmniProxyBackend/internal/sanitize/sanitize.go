package sanitize

import "regexp"

var redactors = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)(authorization\s*:\s*bearer\s+)[^\s,;&]+`), `${1}***`},
	{regexp.MustCompile(`(?i)(access_token|refresh_token|id_token|api[_-]?key|authorization|token|key)=([^&\s]+)`), `${1}=***`},
	{regexp.MustCompile(`(?i)\b(sk-[a-z0-9_-]{8,})\b`), `sk-***`},
	{regexp.MustCompile(`(?i)\b(tp-[a-z0-9_-]{8,})\b`), `tp-***`},
	{regexp.MustCompile(`(?i)\b(bearer\s+)[a-z0-9._~+/=-]{12,}\b`), `${1}***`},
}

func Text(value string) string {
	for _, redactor := range redactors {
		value = redactor.pattern.ReplaceAllString(value, redactor.replacement)
	}
	return value
}
