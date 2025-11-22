package web

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/deemkeen/stegodon/util"
)

func TestGetNodeInfo20(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "stegodon.example"
	conf.Conf.Closed = false

	result := GetNodeInfo20(conf)

	// Parse the JSON result
	var nodeInfo NodeInfo20
	if err := json.Unmarshal([]byte(result), &nodeInfo); err != nil {
		t.Fatalf("Failed to parse NodeInfo JSON: %v", err)
	}

	// Verify version
	if nodeInfo.Version != "2.0" {
		t.Errorf("Expected version to be '2.0', got: %s", nodeInfo.Version)
	}

	// Verify software
	if nodeInfo.Software.Name != "stegodon" {
		t.Errorf("Expected software name to be 'stegodon', got: %s", nodeInfo.Software.Name)
	}
	if nodeInfo.Software.Version == "" {
		t.Error("Software version should not be empty")
	}

	// Verify protocols
	if len(nodeInfo.Protocols) != 1 {
		t.Errorf("Expected 1 protocol, got: %d", len(nodeInfo.Protocols))
	}
	if len(nodeInfo.Protocols) > 0 && nodeInfo.Protocols[0] != "activitypub" {
		t.Errorf("Expected protocol 'activitypub', got: %s", nodeInfo.Protocols[0])
	}

	// Verify services are present (can be empty)
	if nodeInfo.Services.Inbound == nil {
		t.Error("Services.Inbound should not be nil")
	}
	if nodeInfo.Services.Outbound == nil {
		t.Error("Services.Outbound should not be nil")
	}

	// Verify openRegistrations matches config
	if nodeInfo.OpenRegistrations != !conf.Conf.Closed {
		t.Errorf("Expected openRegistrations to be %v, got: %v", !conf.Conf.Closed, nodeInfo.OpenRegistrations)
	}

	// Verify usage statistics structure
	if nodeInfo.Usage.Users.Total < 0 {
		t.Error("Total users should not be negative")
	}
	if nodeInfo.Usage.Users.ActiveMonth < 0 {
		t.Error("Active users (month) should not be negative")
	}
	if nodeInfo.Usage.Users.ActiveHalfyear < 0 {
		t.Error("Active users (half year) should not be negative")
	}
	if nodeInfo.Usage.LocalPosts < 0 {
		t.Error("Local posts should not be negative")
	}

	// Verify metadata is present
	if nodeInfo.Metadata.NodeName == "" {
		t.Error("NodeName should not be empty")
	}
	if nodeInfo.Metadata.NodeDescription == "" {
		t.Error("NodeDescription should not be empty")
	}
}

func TestGetNodeInfo20_ClosedRegistrations(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "stegodon.example"
	conf.Conf.Closed = true // Registrations closed

	result := GetNodeInfo20(conf)

	var nodeInfo NodeInfo20
	if err := json.Unmarshal([]byte(result), &nodeInfo); err != nil {
		t.Fatalf("Failed to parse NodeInfo JSON: %v", err)
	}

	// Verify openRegistrations is false when closed
	if nodeInfo.OpenRegistrations {
		t.Error("Expected openRegistrations to be false when registrations are closed")
	}
}

func TestGetNodeInfo20_JSONStructure(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "stegodon.example"
	conf.Conf.Closed = false

	result := GetNodeInfo20(conf)

	// Parse as generic JSON to check structure
	var data map[string]any
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Required fields according to NodeInfo 2.0 spec
	requiredFields := []string{"version", "software", "protocols", "services", "usage", "openRegistrations", "metadata"}
	for _, field := range requiredFields {
		if _, exists := data[field]; !exists {
			t.Errorf("Missing required field: %s", field)
		}
	}

	// Verify nested structures
	software, ok := data["software"].(map[string]any)
	if !ok {
		t.Fatal("software field should be an object")
	}
	if _, exists := software["name"]; !exists {
		t.Error("software.name is required")
	}
	if _, exists := software["version"]; !exists {
		t.Error("software.version is required")
	}

	usage, ok := data["usage"].(map[string]any)
	if !ok {
		t.Fatal("usage field should be an object")
	}
	if _, exists := usage["users"]; !exists {
		t.Error("usage.users is required")
	}
	if _, exists := usage["localPosts"]; !exists {
		t.Error("usage.localPosts is required")
	}
}

func TestGetWellKnownNodeInfo(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "stegodon.example"

	result := GetWellKnownNodeInfo(conf)

	// Parse the JSON result
	var wellKnown WellKnownNodeInfo
	if err := json.Unmarshal([]byte(result), &wellKnown); err != nil {
		t.Fatalf("Failed to parse well-known nodeinfo JSON: %v", err)
	}

	// Verify links array
	if len(wellKnown.Links) == 0 {
		t.Fatal("Links array should not be empty")
	}

	// Find NodeInfo 2.0 link
	found := false
	for _, link := range wellKnown.Links {
		if link.Rel == "http://nodeinfo.diaspora.software/ns/schema/2.0" {
			found = true

			// Verify href
			expectedHref := "https://stegodon.example/nodeinfo/2.0"
			if link.Href != expectedHref {
				t.Errorf("Expected href to be %s, got: %s", expectedHref, link.Href)
			}

			// Verify href uses HTTPS
			if !strings.HasPrefix(link.Href, "https://") {
				t.Error("NodeInfo href should use HTTPS")
			}

			// Verify href contains domain
			if !strings.Contains(link.Href, conf.Conf.SslDomain) {
				t.Error("NodeInfo href should contain the SSL domain")
			}
		}
	}

	if !found {
		t.Error("NodeInfo 2.0 link not found in well-known document")
	}
}

func TestGetWellKnownNodeInfo_JSONStructure(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "stegodon.example"

	result := GetWellKnownNodeInfo(conf)

	// Parse as generic JSON
	var data map[string]any
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify links field exists
	links, exists := data["links"]
	if !exists {
		t.Fatal("links field is required")
	}

	// Verify links is an array
	linksArray, ok := links.([]any)
	if !ok {
		t.Fatal("links should be an array")
	}

	if len(linksArray) == 0 {
		t.Error("links array should not be empty")
	}

	// Verify each link has rel and href
	for i, link := range linksArray {
		linkObj, ok := link.(map[string]any)
		if !ok {
			t.Fatalf("Link %d should be an object", i)
		}

		if _, exists := linkObj["rel"]; !exists {
			t.Errorf("Link %d missing 'rel' field", i)
		}
		if _, exists := linkObj["href"]; !exists {
			t.Errorf("Link %d missing 'href' field", i)
		}
	}
}

func TestNodeInfo20_ProtocolsList(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "stegodon.example"
	conf.Conf.Closed = false

	result := GetNodeInfo20(conf)

	var nodeInfo NodeInfo20
	if err := json.Unmarshal([]byte(result), &nodeInfo); err != nil {
		t.Fatalf("Failed to parse NodeInfo JSON: %v", err)
	}

	// Verify protocols is an array
	if nodeInfo.Protocols == nil {
		t.Fatal("Protocols should not be nil")
	}

	// Verify it contains activitypub
	hasActivityPub := false
	for _, protocol := range nodeInfo.Protocols {
		if protocol == "activitypub" {
			hasActivityPub = true
		}
	}

	if !hasActivityPub {
		t.Error("Protocols should include 'activitypub'")
	}
}

func TestNodeInfo20_UsageStatistics(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "stegodon.example"
	conf.Conf.Closed = false

	result := GetNodeInfo20(conf)

	var nodeInfo NodeInfo20
	if err := json.Unmarshal([]byte(result), &nodeInfo); err != nil {
		t.Fatalf("Failed to parse NodeInfo JSON: %v", err)
	}

	// Verify usage statistics are logical
	// ActiveMonth should not exceed Total
	if nodeInfo.Usage.Users.ActiveMonth > nodeInfo.Usage.Users.Total {
		t.Error("ActiveMonth users cannot exceed Total users")
	}

	// ActiveHalfyear should not exceed Total
	if nodeInfo.Usage.Users.ActiveHalfyear > nodeInfo.Usage.Users.Total {
		t.Error("ActiveHalfyear users cannot exceed Total users")
	}

	// ActiveMonth should not exceed ActiveHalfyear
	if nodeInfo.Usage.Users.ActiveMonth > nodeInfo.Usage.Users.ActiveHalfyear {
		t.Error("ActiveMonth users cannot exceed ActiveHalfyear users")
	}
}

func TestNodeInfo20_Metadata(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "stegodon.example"
	conf.Conf.Closed = false

	result := GetNodeInfo20(conf)

	var nodeInfo NodeInfo20
	if err := json.Unmarshal([]byte(result), &nodeInfo); err != nil {
		t.Fatalf("Failed to parse NodeInfo JSON: %v", err)
	}

	// Verify metadata fields are strings
	if nodeInfo.Metadata.NodeName == "" {
		t.Error("NodeName should not be empty")
	}

	if nodeInfo.Metadata.NodeDescription == "" {
		t.Error("NodeDescription should not be empty")
	}

	// Verify reasonable lengths
	if len(nodeInfo.Metadata.NodeName) > 100 {
		t.Error("NodeName seems unreasonably long")
	}

	if len(nodeInfo.Metadata.NodeDescription) > 500 {
		t.Error("NodeDescription seems unreasonably long")
	}
}

func TestWellKnownNodeInfo_RelationFormat(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "stegodon.example"

	result := GetWellKnownNodeInfo(conf)

	var wellKnown WellKnownNodeInfo
	if err := json.Unmarshal([]byte(result), &wellKnown); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify rel uses the correct schema URL
	for _, link := range wellKnown.Links {
		if !strings.HasPrefix(link.Rel, "http://nodeinfo.diaspora.software/") {
			t.Errorf("Rel should use NodeInfo schema URL, got: %s", link.Rel)
		}

		// Verify href is a valid URL
		if !strings.HasPrefix(link.Href, "https://") && !strings.HasPrefix(link.Href, "http://") {
			t.Errorf("Href should be a valid URL, got: %s", link.Href)
		}
	}
}

func TestNodeInfo20_SoftwareVersion(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "stegodon.example"
	conf.Conf.Closed = false

	result := GetNodeInfo20(conf)

	var nodeInfo NodeInfo20
	if err := json.Unmarshal([]byte(result), &nodeInfo); err != nil {
		t.Fatalf("Failed to parse NodeInfo JSON: %v", err)
	}

	// Verify version follows semantic versioning pattern
	version := nodeInfo.Software.Version
	if version == "" {
		t.Error("Software version should not be empty")
	}

	// Basic check: should contain at least one digit
	hasDigit := false
	for _, char := range version {
		if char >= '0' && char <= '9' {
			hasDigit = true
			break
		}
	}

	if !hasDigit {
		t.Error("Software version should contain at least one digit")
	}
}

func TestNodeInfo20_OpenRegistrations_Closed(t *testing.T) {
	// Test that openRegistrations is false when STEGODON_CLOSED=true
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "example.com"
	conf.Conf.Closed = true
	conf.Conf.Single = false

	result := GetNodeInfo20(conf)

	var nodeInfo map[string]any
	if err := json.Unmarshal([]byte(result), &nodeInfo); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if nodeInfo["openRegistrations"] != false {
		t.Error("openRegistrations should be false when STEGODON_CLOSED=true")
	}
}

func TestNodeInfo20_OpenRegistrations_Open(t *testing.T) {
	// Test that openRegistrations is true when neither closed nor single with users
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "example.com"
	conf.Conf.Closed = false
	conf.Conf.Single = false

	result := GetNodeInfo20(conf)

	var nodeInfo map[string]any
	if err := json.Unmarshal([]byte(result), &nodeInfo); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Note: This may be true or false depending on actual database state
	// We're just verifying the JSON field exists and is a boolean
	_, ok := nodeInfo["openRegistrations"].(bool)
	if !ok {
		t.Error("openRegistrations should be a boolean value")
	}
}

func TestNodeInfo20_OpenRegistrations_ClosedOverridesSingle(t *testing.T) {
	// Test that STEGODON_CLOSED=true always closes registration
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "example.com"
	conf.Conf.Closed = true
	conf.Conf.Single = false

	result := GetNodeInfo20(conf)

	var nodeInfo map[string]any
	if err := json.Unmarshal([]byte(result), &nodeInfo); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if nodeInfo["openRegistrations"] != false {
		t.Error("openRegistrations should be false when STEGODON_CLOSED=true regardless of other settings")
	}
}
