package header

import (
	"strings"
	"testing"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
)

func TestGetHeaderStyle_NoNotifications(t *testing.T) {
	acc := &domain.Account{
		Username:  "testuser",
		CreatedAt: time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC),
	}
	width := 120
	unreadCount := 0

	result := GetHeaderStyle(acc, width, unreadCount)

	// Should contain username
	if !strings.Contains(result, "testuser") {
		t.Errorf("Header should contain username, got: %s", result)
	}

	// Should contain version
	if !strings.Contains(result, "stegodon v") {
		t.Errorf("Header should contain version, got: %s", result)
	}

	// Should contain join date
	if !strings.Contains(result, "2025-12-10") {
		t.Errorf("Header should contain join date, got: %s", result)
	}

	// Should NOT contain notification badge
	if strings.Contains(result, "[") && strings.Contains(result, "]") {
		// Check it's not the badge format [N]
		if strings.Contains(result, "[0]") || strings.Contains(result, "[1]") {
			t.Errorf("Header should not contain notification badge when count is 0")
		}
	}
}

func TestGetHeaderStyle_WithNotifications(t *testing.T) {
	acc := &domain.Account{
		Username:  "testuser",
		CreatedAt: time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC),
	}
	width := 120
	unreadCount := 5

	result := GetHeaderStyle(acc, width, unreadCount)

	// Should contain username
	if !strings.Contains(result, "testuser") {
		t.Errorf("Header should contain username, got: %s", result)
	}

	// Should contain notification badge with count
	if !strings.Contains(result, "[5]") {
		t.Errorf("Header should contain notification badge [5], got: %s", result)
	}

	// Badge should have warning color ANSI codes
	if !strings.Contains(result, common.ANSI_WARNING_START) {
		t.Errorf("Badge should have warning color ANSI code")
	}

	// Should have color reset after badge
	if !strings.Contains(result, common.ANSI_COLOR_RESET) {
		t.Errorf("Badge should be followed by color reset ANSI code")
	}
}

func TestGetHeaderStyle_BackgroundContinuesAfterBadge(t *testing.T) {
	acc := &domain.Account{
		Username:  "testuser",
		CreatedAt: time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC),
	}
	width := 120
	unreadCount := 3

	result := GetHeaderStyle(acc, width, unreadCount)

	// The header should have ANSI background codes
	// Lipgloss might use different ANSI format, just check some background code exists
	if !strings.Contains(result, "\033[") {
		t.Errorf("Header should have ANSI codes")
	}

	// Find position of badge and verify background continues after it
	badgePos := strings.Index(result, "[3]")
	if badgePos == -1 {
		t.Errorf("Badge [3] not found in header")
		return
	}

	// Check that there's content after the badge (should be spaces and more text)
	afterBadge := result[badgePos+3:]
	if len(afterBadge) < 10 {
		t.Errorf("Header should have substantial content after badge")
	}

	// The background should not be reset immediately after the badge
	// (we use ANSI_COLOR_RESET which is \033[39m, not background reset)
	immediatelyAfter := result[badgePos+3 : min(len(result), badgePos+10)]
	if strings.Contains(immediatelyAfter, "\033[49m") {
		t.Errorf("Background should not be reset immediately after badge")
	}
}

func TestGetHeaderStyle_WidthHandling(t *testing.T) {
	acc := &domain.Account{
		Username:  "testuser",
		CreatedAt: time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC),
	}

	// Test with different widths
	widths := []int{80, 120, 150}
	for _, width := range widths {
		result := GetHeaderStyle(acc, width, 0)

		// Header should contain the main elements regardless of width
		if !strings.Contains(result, "testuser") {
			t.Errorf("Header with width %d should contain username", width)
		}
		if !strings.Contains(result, "stegodon v") {
			t.Errorf("Header with width %d should contain version", width)
		}
	}
}

func TestGetHeaderStyle_LargeNotificationCount(t *testing.T) {
	acc := &domain.Account{
		Username:  "testuser",
		CreatedAt: time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC),
	}
	width := 120
	unreadCount := 99

	result := GetHeaderStyle(acc, width, unreadCount)

	// Should handle large notification counts
	if !strings.Contains(result, "[99]") {
		t.Errorf("Header should contain notification badge [99], got: %s", result)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
