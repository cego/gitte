package startup

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/cego/gitte/executor"
)

type checkResult struct {
	err     error
	elapsed time.Duration
}

type checkSummary struct {
	mu      sync.Mutex
	results map[string]checkResult
}

func newCheckSummary() *checkSummary {
	return &checkSummary{results: make(map[string]checkResult)}
}

func (s *checkSummary) record(name string, err error, elapsed time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[name] = checkResult{err: err, elapsed: elapsed}
}

func (s *checkSummary) render(styled bool, width int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var failed []string
	passed, blocked := 0, 0
	for name, result := range s.results {
		switch {
		case result.err == nil:
			passed++
		case errors.Is(result.err, executor.ErrTaskSkipped):
			blocked++
		default:
			failed = append(failed, name)
		}
	}
	if len(failed) == 0 && blocked == 0 {
		return ""
	}
	sort.Strings(failed)
	var b strings.Builder
	separator := ", "
	if styled {
		separator = " · "
	}
	fmt.Fprintf(&b, "\nStartup: %d failed%s%d blocked%s%d passed\n", len(failed), separator, blocked, separator, passed)
	for _, name := range failed {
		result := s.results[name]
		fmt.Fprintf(&b, "\n%s %s  %s\n", styleText("FAILED", failStyle.Bold(true), styled), styleText(cleanText(name), labelStyle.Bold(true), styled), styleText(fmtDuration(result.elapsed), dimStyle, styled))
		b.WriteString(indentText(wrapText(cleanText(result.err.Error()), max(20, width-2)), "  "))
		var failure *checkFailure
		if errors.As(result.err, &failure) && failure.hint != "" {
			fmt.Fprintf(&b, "\n  %s\n", styleText("How to fix", hintLabelStyle, styled))
			b.WriteString(renderGuidance(failure.hint, styled, width-2, failure.markdown))
		}
	}
	return b.String()
}

func styleText(text string, style lipgloss.Style, styled bool) string {
	if styled {
		return style.Render(text)
	}
	return text
}

func cleanText(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, stripEscapes(text))
}

func indentText(text, prefix string) string {
	return prefix + strings.ReplaceAll(text, "\n", "\n"+prefix) + "\n"
}

// stripEscapes removes terminal control sequences before rendering authored text.
func stripEscapes(text string) string {
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] != '\x1b' {
			b.WriteByte(text[i])
			continue
		}
		i++
		if i >= len(text) {
			break
		}
		switch text[i] {
		case '[':
			for i++; i < len(text); i++ {
				if text[i] >= 0x40 && text[i] <= 0x7e {
					break
				}
			}
		case ']', 'P', 'X', '^', '_':
			for i++; i < len(text); i++ {
				if text[i] == '\a' {
					break
				}
				if text[i] == '\x1b' && i+1 < len(text) && text[i+1] == '\\' {
					i++
					break
				}
			}
		default:
			for i < len(text) && text[i] >= 0x20 && text[i] <= 0x2f {
				i++
			}
		}
	}
	return b.String()
}

// wrapText wraps prose at whitespace, leaving long paths and tokens intact.
func wrapText(text string, width int) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) <= width {
			continue
		}
		var b strings.Builder
		column := 0
		for _, word := range strings.Fields(line) {
			wordWidth := lipgloss.Width(word)
			if column > 0 {
				if column+1+wordWidth > width {
					b.WriteByte('\n')
					column = 0
				} else {
					b.WriteByte(' ')
					column++
				}
			}
			b.WriteString(word)
			column += wordWidth
		}
		lines[i] = b.String()
	}
	return strings.Join(lines, "\n")
}
