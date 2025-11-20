package web

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/deemkeen/stegodon/util"
	"github.com/gin-gonic/gin"
)

// DeltaMeshWellKnown represents the discovery document advertised from /.well-known/deltamesh.
// The structure is intentionally small so we can evolve it alongside the protocol draft once
// the full specification is available locally.
type DeltaMeshWellKnown struct {
	Version        string            `json:"version"`
	Implementation string            `json:"implementation"`
	Node           string            `json:"node"`
	Capabilities   []string          `json:"capabilities"`
	Endpoints      map[string]string `json:"endpoints"`
}

// DeltaMeshNode describes the current node status for quick health checks.
type DeltaMeshNode struct {
	Node           string    `json:"node"`
	Implementation string    `json:"implementation"`
	Version        string    `json:"version"`
	GeneratedAt    time.Time `json:"generatedAt"`
}

// DeltaMeshRepositoryCatalog is a placeholder collection of repositories advertised by this node.
// The spec in the git-federation branch calls for a catalog resource; we ship an empty catalog
// that can be filled in once repository exposure rules are finalized.
type DeltaMeshRepositoryCatalog struct {
	Repositories []string `json:"repositories"`
}

func registerDeltaMeshRoutes(g *gin.Engine, conf *util.AppConfig) {
	g.GET("/.well-known/deltamesh", func(c *gin.Context) {
		base := resolvePublicBase(conf, c.Request)

		c.JSON(200, DeltaMeshWellKnown{
			Version:        "draft-01",
			Implementation: "stegodon",
			Node:           base,
			Capabilities:   []string{"catalog", "status"},
			Endpoints: map[string]string{
				"node":      fmt.Sprintf("%s/deltamesh/node", base),
				"catalog":   fmt.Sprintf("%s/deltamesh/repos", base),
				"wellKnown": fmt.Sprintf("%s/.well-known/deltamesh", base),
			},
		})
	})

	g.GET("/deltamesh/node", func(c *gin.Context) {
		base := resolvePublicBase(conf, c.Request)
		c.JSON(200, DeltaMeshNode{
			Node:           base,
			Implementation: "stegodon",
			Version:        "draft-01",
			GeneratedAt:    time.Now().UTC(),
		})
	})

	g.GET("/deltamesh/repos", func(c *gin.Context) {
		c.JSON(200, DeltaMeshRepositoryCatalog{Repositories: []string{}})
	})
}

func resolvePublicBase(conf *util.AppConfig, req *http.Request) string {
	scheme, host := requestOrigin(req)
	if scheme != "" && host != "" {
		return fmt.Sprintf("%s://%s", scheme, host)
	}

	if conf == nil {
		return ""
	}

	if conf.Conf.SslDomain != "" {
		return fmt.Sprintf("https://%s", conf.Conf.SslDomain)
	}

	host = conf.Conf.Host
	if host == "" {
		host = "localhost"
	}

	port := conf.Conf.HttpPort
	if port <= 0 {
		port = 80
	}

	// Preserve literal host if it already includes a port to avoid double-appending.
	if strings.Contains(host, ":") && strings.Count(host, ":") == 1 {
		return fmt.Sprintf("http://%s", host)
	}

	return fmt.Sprintf("http://%s", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
}

func requestOrigin(req *http.Request) (string, string) {
	if req == nil {
		return "", ""
	}

	proto := strings.TrimSpace(req.Header.Get("X-Forwarded-Proto"))
	host := strings.TrimSpace(req.Header.Get("X-Forwarded-Host"))

	if proto == "" {
		if req.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}

	if host == "" {
		host = req.Host
	}

	return proto, host
}
