package notifications

import (
	"fmt"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
)

const (
	notificationsLimit = 50
	refreshInterval    = 30 * time.Second
)

type Model struct {
	AccountId     uuid.UUID
	Notifications []domain.Notification
	Selected      int
	Offset        int
	Width         int
	Height        int
	isActive      bool
	UnreadCount   int
}

type notificationsLoadedMsg struct {
	notifications []domain.Notification
	unreadCount   int
}

type refreshTickMsg struct{}

func InitialModel(accountId uuid.UUID, width, height int) Model {
	return Model{
		AccountId:     accountId,
		Notifications: []domain.Notification{},
		Selected:      0,
		Offset:        0,
		Width:         width,
		Height:        height,
		isActive:      false,
		UnreadCount:   0,
	}
}

func (m Model) Init() tea.Cmd {
	return nil // Don't start commands - model starts inactive
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case common.ActivateViewMsg:
		log.Printf("Notifications: Received ActivateViewMsg, AccountId=%s", m.AccountId)
		m.isActive = true
		return m, tea.Batch(loadNotifications(m.AccountId), tickRefresh())

	case common.DeactivateViewMsg:
		log.Printf("Notifications: Received DeactivateViewMsg")
		m.isActive = false
		return m, nil

	case notificationsLoadedMsg:
		log.Printf("Notifications: Loaded %d notifications, unread=%d", len(msg.notifications), msg.unreadCount)
		m.Notifications = msg.notifications
		m.UnreadCount = msg.unreadCount
		// Keep selection within bounds
		if m.Selected >= len(m.Notifications) {
			m.Selected = len(m.Notifications) - 1
		}
		if m.Selected < 0 {
			m.Selected = 0
		}
		return m, nil

	case refreshTickMsg:
		if m.isActive {
			return m, tea.Batch(loadNotifications(m.AccountId), tickRefresh())
		}
		return m, nil // Stop ticker chain when inactive

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.Selected > 0 {
				m.Selected--
				// Scroll up if needed
				if m.Selected < m.Offset {
					m.Offset = m.Selected
				}
			}
		case "down", "j":
			if m.Selected < len(m.Notifications)-1 {
				m.Selected++
				// Scroll down if needed
				itemsPerPage := common.DefaultItemsPerPage
				if m.Selected >= m.Offset+itemsPerPage {
					m.Offset = m.Selected - itemsPerPage + 1
				}
			}
		case "enter":
			// Mark notification as read and navigate to source
			if m.Selected < len(m.Notifications) {
				notif := m.Notifications[m.Selected]
				if !notif.Read {
					return m, markNotificationRead(notif.Id)
				}
				// TODO: Navigate to thread or profile based on notification type
			}
		case "a":
			// Mark all as read
			return m, markAllNotificationsRead(m.AccountId)
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	// Header
	title := fmt.Sprintf("🔔 Notifications (%d unread)", m.UnreadCount)
	s.WriteString(common.CaptionStyle.Render(title))
	s.WriteString("\n\n")

	log.Printf("Notifications View: count=%d, isActive=%v", len(m.Notifications), m.isActive)

	if len(m.Notifications) == 0 {
		s.WriteString(common.ListEmptyStyle.Render("No notifications yet."))
		return s.String()
	}

	// Calculate visible range
	itemsPerPage := common.DefaultItemsPerPage
	start := m.Offset
	end := start + itemsPerPage
	if end > len(m.Notifications) {
		end = len(m.Notifications)
	}

	// Render notifications
	for i := start; i < end; i++ {
		notif := m.Notifications[i]
		selected := i == m.Selected

		// Format notification
		line1 := fmt.Sprintf("%s %s %s", notif.TypeIcon(), notif.ActorHandle(), notif.TypeLabel())
		timeAgo := formatTimeAgo(notif.CreatedAt)

		if selected {
			// Selected styling
			if !notif.Read {
				s.WriteString(common.ListSelectedPrefix +
					lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(common.COLOR_USERNAME)).Render(line1) +
					"  " + common.ListBadgeStyle.Render(timeAgo))
			} else {
				s.WriteString(common.ListSelectedPrefix +
					common.ListItemSelectedStyle.Render(line1) +
					"  " + common.ListBadgeStyle.Render(timeAgo))
			}
		} else {
			// Normal styling
			if !notif.Read {
				s.WriteString(common.ListUnselectedPrefix +
					lipgloss.NewStyle().Bold(true).Render(line1) +
					"  " + common.ListBadgeStyle.Render(timeAgo))
			} else {
				s.WriteString(common.ListUnselectedPrefix +
					common.ListItemStyle.Render(line1) +
					"  " + common.ListBadgeStyle.Render(timeAgo))
			}
		}
		s.WriteString("\n")

		// Show preview for like/reply/mention (indented)
		if notif.NotePreview != "" && notif.NotificationType != domain.NotificationFollow {
			preview := truncate(notif.NotePreview, 60)
			s.WriteString("  " + common.ListBadgeStyle.Render("\""+preview+"\""))
			s.WriteString("\n")
		}
	}

	// Pagination info
	if len(m.Notifications) > itemsPerPage {
		pageInfo := fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(m.Notifications))
		s.WriteString("\n" + common.ListBadgeStyle.Render(pageInfo))
	}

	return s.String()
}

// loadNotifications loads notifications for an account
func loadNotifications(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		log.Printf("loadNotifications: Starting for AccountId=%s", accountId)
		database := db.GetDB()
		err, notifications := database.ReadNotificationsByAccountId(accountId, notificationsLimit)
		if err != nil {
			log.Printf("Failed to load notifications: %v", err)
			return notificationsLoadedMsg{notifications: []domain.Notification{}, unreadCount: 0}
		}

		log.Printf("loadNotifications: Got %d notifications", len(*notifications))

		// Get unread count
		unreadCount, err := database.ReadUnreadNotificationCount(accountId)
		if err != nil {
			log.Printf("Failed to get unread count: %v", err)
			unreadCount = 0
		}

		log.Printf("loadNotifications: Unread count=%d", unreadCount)

		return notificationsLoadedMsg{
			notifications: *notifications,
			unreadCount:   unreadCount,
		}
	}
}

// markNotificationRead marks a notification as read
func markNotificationRead(notificationId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		if err := database.MarkNotificationRead(notificationId); err != nil {
			log.Printf("Failed to mark notification as read: %v", err)
		}
		return nil
	}
}

// markAllNotificationsRead marks all notifications for an account as read
func markAllNotificationsRead(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		if err := database.MarkAllNotificationsRead(accountId); err != nil {
			log.Printf("Failed to mark all notifications as read: %v", err)
		}
		// Reload notifications to update the view
		return loadNotifications(accountId)()
	}
}

// tickRefresh returns a command that triggers a refresh after a delay
func tickRefresh() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

// formatTimeAgo formats a time as a relative string (e.g., "2h ago")
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%dm ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%dh ago", hours)
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	} else if duration < 30*24*time.Hour {
		weeks := int(duration.Hours() / 24 / 7)
		return fmt.Sprintf("%dw ago", weeks)
	} else if duration < 365*24*time.Hour {
		months := int(duration.Hours() / 24 / 30)
		return fmt.Sprintf("%dmo ago", months)
	} else {
		years := int(duration.Hours() / 24 / 365)
		return fmt.Sprintf("%dy ago", years)
	}
}

// truncate truncates a string to a maximum length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
