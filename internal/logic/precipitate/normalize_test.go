package precipitate

import (
	"strings"
	"testing"
)

func TestNormalizeQuestion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "spaces", input: "  今天车流怎么样  ", want: "今天车流怎么样"},
		{name: "uppercase", input: "SELECT * FROM users", want: "select * from users"},
		{name: "chinese punctuation", input: "今天车流是多少？", want: "今天车流是多少"},
		{name: "mixed punctuation", input: "车流增长10%！", want: "车流增长10%"},
		{name: "long text truncation", input: strings.Repeat("a", 200), want: strings.Repeat("a", 120)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeQuestion(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeQuestion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
