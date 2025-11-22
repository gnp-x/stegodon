package web

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/util"
)

// NodeInfo20 represents the NodeInfo 2.0 schema
// See: https://nodeinfo.diaspora.software/schema.html
type NodeInfo20 struct {
	Version           string           `json:"version"`
	Software          NodeInfoSoftware `json:"software"`
	Protocols         []string         `json:"protocols"`
	Services          NodeInfoServices `json:"services"`
	OpenRegistrations bool             `json:"openRegistrations"`
	Usage             NodeInfoUsage    `json:"usage"`
	Metadata          NodeInfoMetadata `json:"metadata"`
}

type NodeInfoSoftware struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type NodeInfoServices struct {
	Inbound  []string `json:"inbound"`
	Outbound []string `json:"outbound"`
}

type NodeInfoUsage struct {
	Users      NodeInfoUsers `json:"users"`
	LocalPosts int           `json:"localPosts"`
}

type NodeInfoUsers struct {
	Total          int `json:"total"`
	ActiveMonth    int `json:"activeMonth"`
	ActiveHalfyear int `json:"activeHalfyear"`
}

type NodeInfoMetadata struct {
	NodeName        string `json:"nodeName"`
	NodeDescription string `json:"nodeDescription"`
}

// WellKnownNodeInfo represents the /.well-known/nodeinfo response
type WellKnownNodeInfo struct {
	Links []NodeInfoLink `json:"links"`
}

type NodeInfoLink struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

// GetNodeInfo20 returns a NodeInfo 2.0 JSON response
func GetNodeInfo20(conf *util.AppConfig) string {
	database := db.GetDB()

	// Get statistics from database
	totalUsers, err := database.CountAccounts()
	if err != nil {
		log.Printf("Failed to count accounts: %v", err)
		totalUsers = 0
	}

	localPosts, err := database.CountLocalPosts()
	if err != nil {
		log.Printf("Failed to count local posts: %v", err)
		localPosts = 0
	}

	activeMonth, err := database.CountActiveUsersMonth()
	if err != nil {
		log.Printf("Failed to count active users (month): %v", err)
		activeMonth = 0
	}

	activeHalfyear, err := database.CountActiveUsersHalfYear()
	if err != nil {
		log.Printf("Failed to count active users (half year): %v", err)
		activeHalfyear = 0
	}

	// Determine if registrations are open
	// Closed if: STEGODON_CLOSED=true OR (STEGODON_SINGLE=true AND user exists)
	openRegistrations := !conf.Conf.Closed
	if conf.Conf.Single && totalUsers >= 1 {
		openRegistrations = false
	}

	// Get node description (use custom if set, otherwise default)
	nodeDescription := conf.Conf.NodeDescription
	if nodeDescription == "" {
		nodeDescription = "A SSH-first federated microblog"
	}

	// Build NodeInfo response using json.RawMessage to preserve field order
	// We use a slice of key-value pairs to maintain exact order
	nodeInfoJSON := fmt.Sprintf(`{
  "version": "2.0",
  "software": {
    "name": "stegodon",
    "version": "%s"
  },
  "protocols": ["activitypub"],
  "services": {
    "outbound": [],
    "inbound": []
  },
  "usage": {
    "users": {
      "total": %d,
      "activeMonth": %d,
      "activeHalfyear": %d
    },
    "localPosts": %d
  },
  "openRegistrations": %t,
  "metadata": {
    "nodeName": "Stegodon",
    "nodeDescription": "%s"
  }
}`,
		util.GetVersion(),
		totalUsers,
		activeMonth,
		activeHalfyear,
		localPosts,
		openRegistrations,
		nodeDescription,
	)

	return nodeInfoJSON
}

// GetWellKnownNodeInfo returns the /.well-known/nodeinfo discovery document
func GetWellKnownNodeInfo(conf *util.AppConfig) string {
	wellKnown := WellKnownNodeInfo{
		Links: []NodeInfoLink{
			{
				Rel:  "http://nodeinfo.diaspora.software/ns/schema/2.0",
				Href: "https://" + conf.Conf.SslDomain + "/nodeinfo/2.0",
			},
		},
	}

	jsonBytes, err := json.Marshal(wellKnown)
	if err != nil {
		log.Printf("Failed to marshal well-known nodeinfo: %v", err)
		return "{}"
	}

	return string(jsonBytes)
}
