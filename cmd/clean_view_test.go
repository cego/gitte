package cmd

import (
	"strings"
	"testing"
)

func TestRenderPhaseHeader(t *testing.T) {
	// header fills to width (ignoring ANSI codes)
	got := renderPhaseHeader("Untracked", 40)
	// "── Untracked " is 13 chars; remaining is 27 "─"s = 40 total
	stripped := stripANSIClean(got)
	if len([]rune(stripped)) != 40 {
		t.Errorf("expected width 40, got %d: %q", len([]rune(stripped)), stripped)
	}
	if !strings.HasPrefix(stripped, "── Untracked ") {
		t.Errorf("unexpected prefix: %q", stripped)
	}
}

// stripANSIClean strips ANSI escape codes for testing.
func stripANSIClean(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func TestColsForWidth(t *testing.T) {
	cases := []struct {
		width int
		want  int
	}{
		{79, 1},
		{80, 1},
		{119, 1},
		{120, 2},
		{179, 2},
		{180, 3},
		{300, 3},
	}
	for _, c := range cases {
		got := colsForWidth(c.width)
		if got != c.want {
			t.Errorf("colsForWidth(%d) = %d, want %d", c.width, got, c.want)
		}
	}
}
