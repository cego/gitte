package startup

import (
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
)

var inlineMarkup = regexp.MustCompile("`[^`]+`|\\*\\*[^*]+\\*\\*")
var commandStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51")).TabWidth(lipgloss.NoTabConversion)

// renderGuidance supports paragraphs, lists, bold, inline code and fenced code.
// Code lines are never wrapped so commands remain suitable for copying.
func renderGuidance(text string, styled bool, width int, markdown bool) string {
	var b strings.Builder
	fence := ""
	for _, line := range strings.Split(strings.TrimRight(cleanText(text), "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if markdown {
			if fence != "" && strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]) == "" {
				fence = ""
				continue
			}
			if fence == "" && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")) {
				end := 3
				for end < len(trimmed) && trimmed[end] == trimmed[0] {
					end++
				}
				fence = trimmed[:end]
				continue
			}
		}
		if fence != "" {
			b.WriteString(styleText(line, commandStyle, styled))
			b.WriteByte('\n')
			continue
		}
		if !markdown && (strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t")) {
			b.WriteString(indentText(styleText(line, commandStyle, styled), "    "))
			continue
		}
		line = inlineMarkup.ReplaceAllStringFunc(line, func(token string) string {
			if strings.HasPrefix(token, "`") {
				return styleText(token[1:len(token)-1], commandStyle, styled)
			}
			return styleText(token[2:len(token)-2], labelStyle.Bold(true), styled)
		})
		b.WriteString(indentText(wrapText(line, max(20, width)), "  "))
	}
	return b.String()
}
