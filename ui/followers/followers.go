package followers

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
	"log"
)

type Model struct {
	AccountId uuid.UUID
	Followers []domain.Follow
	Selected  int
	Offset    int // Pagination offset
	Width     int
	Height    int
}

func InitialModel(accountId uuid.UUID, width, height int) Model {
	return Model{
		AccountId: accountId,
		Followers: []domain.Follow{},
		Selected:  0,
		Offset:    0,
		Width:     width,
		Height:    height,
	}
}

func (m Model) Init() tea.Cmd {
	return loadFollowers(m.AccountId)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case followersLoadedMsg:
		m.Followers = msg.followers
		m.Offset = 0
		m.Selected = 0
		return m, nil

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
			if m.Selected < len(m.Followers)-1 {
				m.Selected++
				// Scroll down if needed
				if m.Selected >= m.Offset+common.DefaultItemsPerPage {
					m.Offset = m.Selected - common.DefaultItemsPerPage + 1
				}
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("followers (%d)", len(m.Followers))))
	s.WriteString("\n\n")

	if len(m.Followers) == 0 {
		s.WriteString(common.ListEmptyStyle.Render("No followers yet. Share your profile to get followers!"))
		return s.String()
	}

	start := m.Offset
	end := min(start+common.DefaultItemsPerPage, len(m.Followers))

	for i := start; i < end; i++ {
		follow := m.Followers[i]
		database := db.GetDB()

		var username, badge string

		if follow.IsLocal {
			// Local follower - look up in accounts table
			err, localAcc := database.ReadAccById(follow.AccountId)
			if err != nil {
				log.Printf("Failed to read local account: %v", err)
				continue
			}
			username = "@" + localAcc.Username
			badge = " [local]"
		} else {
			// Remote follower - look up in remote_accounts table
			err, remoteAcc := database.ReadRemoteAccountById(follow.AccountId)
			if err != nil {
				log.Printf("Failed to read remote account: %v", err)
				continue
			}
			username = fmt.Sprintf("@%s@%s", remoteAcc.Username, remoteAcc.Domain)
			badge = ""
		}

		if i == m.Selected {
			// Selected item with arrow prefix
			text := common.ListItemSelectedStyle.Render(username + badge)
			s.WriteString(common.ListSelectedPrefix + text)
		} else {
			// Normal item
			text := username + common.ListBadgeStyle.Render(badge)
			s.WriteString(common.ListUnselectedPrefix + common.ListItemStyle.Render(text))
		}
		s.WriteString("\n")
	}

	// Show pagination info if there are more items
	if len(m.Followers) > common.DefaultItemsPerPage {
		s.WriteString("\n")
		paginationText := fmt.Sprintf("showing %d-%d of %d", start+1, end, len(m.Followers))
		s.WriteString(common.ListBadgeStyle.Render(paginationText))
	}

	return s.String()
}

// followersLoadedMsg is sent when followers are loaded
type followersLoadedMsg struct {
	followers []domain.Follow
}

// loadFollowers loads the followers for the given account
func loadFollowers(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Clean up any orphaned follows before loading
		if err := database.CleanupOrphanedFollows(); err != nil {
			log.Printf("Warning: Failed to cleanup orphaned follows: %v", err)
		}

		err, followers := database.ReadFollowersByAccountId(accountId)
		if err != nil {
			log.Printf("Failed to load followers: %v", err)
			return followersLoadedMsg{followers: []domain.Follow{}}
		}

		if followers == nil {
			return followersLoadedMsg{followers: []domain.Follow{}}
		}

		return followersLoadedMsg{followers: *followers}
	}
}
