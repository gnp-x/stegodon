package common

import "github.com/charmbracelet/lipgloss"

const (
	COLOR_GREY        = "241" // ANSI 241
	COLOR_MAGENTA     = "170" // ANSI 170
	COLOR_LIGHTBLUE   = "69"  // ANSI 69
	COLOR_PURPLE      = "141" // ANSI 141 - closest to #7D56F4
	COLOR_GREEN       = "48"  // ANSI 48 - bright green
	COLOR_BLUE        = "75"  // ANSI 75 - light blue
	COLOR_DARK_GREY   = "240" // ANSI 240 - Muted text
	COLOR_BORDER_GREY = "240" // ANSI 240 - Border color
	COLOR_WHITE       = "255" // ANSI 255 - White text
	COLOR_RED         = "196" // ANSI 196 - Error/warning red
	COLOR_SUCCESS     = "42"  // ANSI 42 - Success green
	COLOR_MUTED       = "245" // ANSI 245 - Muted/disabled text
	COLOR_BLACK       = "0"   // ANSI 0 - Black
	COLOR_CYAN        = "117" // ANSI 117 - Cyan/light blue for highlights
	COLOR_DIM         = "242" // ANSI 242 - Dim text
	COLOR_LIGHT       = "252" // ANSI 252 - Light text
	COLOR_BRIGHT_RED  = "9"   // ANSI 9 - Bright red for critical errors

	// OSC8 hyperlink color (RGB format for true color terminals)
	// Green color: RGB(0, 255, 127) = #00ff7f
	COLOR_LINK_RGB = "0;255;127"
)

var (
	HelpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(COLOR_GREY)).Padding(0, 2)
	CaptionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(COLOR_MAGENTA)).Padding(2)
)

// DefaultWindowWidth returns the usable width after accounting for outer margins
// The offset of 10 accounts for: outer margins (2*2=4) + potential scrollbar (2) + safety buffer (4)
func DefaultWindowWidth(width int) int {
	return width - 10
}

// DefaultWindowHeight returns the usable height after accounting for outer margins
// The offset of 10 accounts for: outer margins (2*2=4) + potential terminal chrome (6)
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
