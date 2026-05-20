package precipitate

import (
	"strings"
)

// NormalizeQuestion normalizes a user question for deduplication:
// trim spaces, lowercase, remove common punctuation, truncate to 120 runes.
func NormalizeQuestion(q string) string {
	q = strings.TrimSpace(q)
	q = strings.ToLower(q)
	replacer := strings.NewReplacer(
		"！", "",
		"？", "",
		"。", "",
		"，", "",
		"、", "",
		"；", "",
		"：", "",
		"“", "",
		"”", "",
		"'", "",
		"【", "",
		"】", "",
		"《", "",
		"》", "",
		"（", "",
		"）", "",
		"?", "",
		"!", "",
		",", "",
		".", "",
		";", "",
		":", "",
		"'", "",
		"\"", "",
	)
	q = replacer.Replace(q)
	runes := []rune(q)
	if len(runes) > 120 {
		runes = runes[:120]
	}
	return string(runes)
}
