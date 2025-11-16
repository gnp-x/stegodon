package common

import "github.com/charmbracelet/lipgloss"

const (
	COLOR_GREY        = "#626262" // ANSI 241
	COLOR_MAGENTA     = "#d75fd7" // ANSI 170
	COLOR_LIGHTBLUE   = "#5f87ff" // ANSI 69
	COLOR_PURPLE      = "#7D56F4" // TrueColor purple
	COLOR_GREEN       = "#00ff7f" // Terminal green accent
	COLOR_BLUE        = "#5fafff" // Link/secondary blue
	COLOR_DARK_GREY   = "#585858" // ANSI 240 - Muted text
	COLOR_BORDER_GREY = "#585858" // ANSI 240 - Border color
	COLOR_WHITE       = "#eeeeee" // ANSI 255 - White text
	COLOR_RED         = "#ff0000" // ANSI 196 - Error/warning red
)

var (
	HelpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(COLOR_GREY)).Padding(0, 2)
	CaptionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(COLOR_MAGENTA)).Padding(2)
)

func DefaultWindowWidth(width int) int {
	return width - 10
}

func DefaultWindowHeight(heigth int) int {
	return heigth - 10
}

func DefaultCreateNoteWidth(width int) int {
	return width / 4
}

func DefaultListWidth(width int) int {
	return width - DefaultCreateNoteWidth(width)
}

func DefaultListHeight(height int) int {
	return height
}
