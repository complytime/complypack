package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Output format constants.
const (
	formatHuman = "human"
	formatText  = "text"
	formatJSON  = "json"
)

var (
	styleTitle   = lipgloss.NewStyle().Bold(true)
	styleControl = lipgloss.NewStyle().Bold(true)
	stylePass    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	styleFail    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleGap     = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	styleWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleDim     = lipgloss.NewStyle().Faint(true)
)

// Renders common header for human-readable reports
func renderHeader(title string) string {
	return styleTitle.Render(title) + "\n" + styleDim.Render(strings.Repeat("━", 50))
}

// Renders common separator for human-readable reports
func renderSeparator() string {
	return styleDim.Render(strings.Repeat("─", 50))
}

// Renders human-readable report metadata
func renderMetadata(key string, value interface{}) string {
	return fmt.Sprintf("%s %v", styleDim.Render(key+":"), value)
}

// resolveFormat determines the output format from the flag value and environment.
// When no flag is provided, it defaults to "text" if NO_COLOR is set, otherwise "human".
func resolveFormat(flagValue string) (string, error) {
	if flagValue != "" {
		switch flagValue {
		case formatHuman, formatText, formatJSON:
			return flagValue, nil
		default:
			return "", fmt.Errorf("unknown format %q; valid formats: human, text, json", flagValue)
		}
	}
	if os.Getenv("NO_COLOR") != "" {
		return formatText, nil
	}
	return formatHuman, nil
}
