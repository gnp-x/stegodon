package admin

import (
	"fmt"
	"strings"

	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
)

type Model struct {
	AdminId  uuid.UUID
	Users    []domain.Account
	Selected int
	Offset   int // Pagination offset
	Width    int
	Height   int
	Status   string
	Error    string
}

func InitialModel(adminId uuid.UUID, width, height int) Model {
	return Model{
		AdminId:  adminId,
		Users:    []domain.Account{},
		Selected: 0,
		Offset:   0,
		Width:    width,
		Height:   height,
		Status:   "",
		Error:    "",
	}
}

func (m Model) Init() tea.Cmd {
	return loadUsers()
}

type usersLoadedMsg struct {
	users []domain.Account
}

type muteUserMsg struct {
	userId uuid.UUID
}

type kickUserMsg struct {
	userId uuid.UUID
}

func loadUsers() tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, users := database.ReadAllAccountsAdmin()
		if err != nil {
			log.Printf("Failed to load users: %v", err)
			return usersLoadedMsg{users: []domain.Account{}}
		}
		if users == nil {
			log.Printf("Failed to load users: users is nil")
			return usersLoadedMsg{users: []domain.Account{}}
		}
		return usersLoadedMsg{users: *users}
	}
}

func muteUser(userId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err := database.MuteUser(userId)
		if err != nil {
			log.Printf("Failed to mute user: %v", err)
		}
		return muteUserMsg{userId: userId}
	}
}

func kickUser(userId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err := database.DeleteAccount(userId)
		if err != nil {
			log.Printf("Failed to kick user: %v", err)
		}
		return kickUserMsg{userId: userId}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case usersLoadedMsg:
		m.Users = msg.users
		m.Selected = 0
		m.Offset = 0
		if m.Selected >= len(m.Users) {
			m.Selected = max(0, len(m.Users)-1)
		}
		return m, nil

	case muteUserMsg:
		m.Status = "User muted and posts deleted"
		m.Error = ""
		return m, loadUsers()

	case kickUserMsg:
		m.Status = "User kicked successfully"
		m.Error = ""
		return m, loadUsers()

	case tea.KeyMsg:
		m.Status = ""
		m.Error = ""

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
			if len(m.Users) > 0 && m.Selected < len(m.Users)-1 {
				m.Selected++
				// Scroll down if needed
				if m.Selected >= m.Offset+common.DefaultItemsPerPage {
					m.Offset = m.Selected - common.DefaultItemsPerPage + 1
				}
			}
		case "m":
			// Mute selected user
			if len(m.Users) > 0 && m.Selected < len(m.Users) {
				selectedUser := m.Users[m.Selected]
				// Can't mute admin or yourself
				if selectedUser.IsAdmin {
					m.Error = "Cannot mute admin user"
					return m, nil
				}
				if selectedUser.Id == m.AdminId {
					m.Error = "Cannot mute yourself"
					return m, nil
				}
				if selectedUser.Muted {
					m.Error = "User is already muted"
					return m, nil
				}
				return m, muteUser(selectedUser.Id)
			}
		case "K":
			// Kick selected user (capital K to prevent accidental kicks)
			if len(m.Users) > 0 && m.Selected < len(m.Users) {
				selectedUser := m.Users[m.Selected]
				// Can't kick admin or yourself
				if selectedUser.IsAdmin {
					m.Error = "Cannot kick admin user"
					return m, nil
				}
				if selectedUser.Id == m.AdminId {
					m.Error = "Cannot kick yourself"
					return m, nil
				}
				return m, kickUser(selectedUser.Id)
			}
		}
	}

	return m, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("admin panel (%d users)", len(m.Users))))
	s.WriteString("\n\n")

	if len(m.Users) == 0 {
		s.WriteString(common.ListEmptyStyle.Render("No users found."))
		return s.String()
	}

	start := m.Offset
	end := min(start+common.DefaultItemsPerPage, len(m.Users))

	for i := start; i < end; i++ {
		user := m.Users[i]

		username := "@" + user.Username
		var badges []string

		if user.IsAdmin {
			badges = append(badges, "[ADMIN]")
		}
		if user.Muted {
			badges = append(badges, "[MUTED]")
		}

		badge := ""
		if len(badges) > 0 {
			badge = " " + strings.Join(badges, " ")
		}

		if i == m.Selected {
			// Selected item with arrow prefix
			text := common.ListItemSelectedStyle.Render(username + badge)
			s.WriteString(common.ListSelectedPrefix + text)
		} else if user.Muted {
			// Muted users shown in error/red color
			text := username + common.ListBadgeMutedStyle.Render(badge)
			s.WriteString(common.ListUnselectedPrefix + common.ListItemStyle.Render(text))
		} else {
			// Normal item
			text := username + common.ListBadgeStyle.Render(badge)
			s.WriteString(common.ListUnselectedPrefix + common.ListItemStyle.Render(text))
		}
		s.WriteString("\n")
	}

	// Show pagination info if there are more items
	if len(m.Users) > common.DefaultItemsPerPage {
		s.WriteString("\n")
		paginationText := fmt.Sprintf("showing %d-%d of %d", start+1, end, len(m.Users))
		s.WriteString(common.ListBadgeStyle.Render(paginationText))
	}

	s.WriteString("\n")

	if m.Status != "" {
		s.WriteString(common.ListStatusStyle.Render(m.Status))
		s.WriteString("\n")
	}

	if m.Error != "" {
		s.WriteString(common.ListErrorStyle.Render("Error: " + m.Error))
		s.WriteString("\n")
	}

	return s.String()
}
