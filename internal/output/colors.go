package output

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Styles for terminal output. These are initialized based on color support.
var (
	LabelStyle   lipgloss.Style
	SuccessStyle lipgloss.Style
	ErrorStyle   lipgloss.Style
	WarningStyle lipgloss.Style
	HeaderStyle  lipgloss.Style
)

func init() {
	initStyles()
}

func initStyles() {
	LabelStyle = lipgloss.NewStyle().Bold(true)
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	ErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))   // red
	WarningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	HeaderStyle = lipgloss.NewStyle().Bold(true).Underline(true)
}

// StatusBadge returns a styled status string with appropriate color.
func StatusBadge(status string) string {
	upper := strings.ToUpper(status)
	switch upper {
	case "SENT", "DELIVERED", "ACTIVE", "RECEIVED":
		return SuccessStyle.Render(upper)
	case "DRAFT", "PENDING", "TRANSIT":
		return WarningStyle.Render(upper)
	case "FAILED", "ERROR", "REJECTED":
		return ErrorStyle.Render(upper)
	default:
		return upper
	}
}

// IsTTY reports whether the given file descriptor is a terminal.
func IsTTY(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

// hasColor reports whether color output should be enabled.
func hasColor(noColorFlag bool, isTTY bool) bool {
	if noColorFlag {
		return false
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	return isTTY
}
