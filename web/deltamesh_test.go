package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deemkeen/stegodon/util"
	"github.com/gin-gonic/gin"
)

func TestResolvePublicBase(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.Host = "127.0.0.1"
	conf.Conf.HttpPort = 9999

	t.Run("config fallback", func(t *testing.T) {
		if got := resolvePublicBase(conf, nil); got != "http://127.0.0.1:9999" {
			t.Fatalf("expected fallback http base, got %s", got)
		}
	})

	t.Run("ssl domain", func(t *testing.T) {
		conf.Conf.SslDomain = "example.com"
		if got := resolvePublicBase(conf, nil); got != "https://example.com" {
			t.Fatalf("expected SSL domain base, got %s", got)
		}
	})

	t.Run("forwarded headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", "forwarded.example")

		if got := resolvePublicBase(conf, req); got != "https://forwarded.example" {
			t.Fatalf("expected forwarded origin, got %s", got)
		}
	})
}

func TestDeltaMeshRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	conf := &util.AppConfig{}
	conf.Conf.Host = "127.0.0.1"
	conf.Conf.HttpPort = 4242

	r := gin.New()
	registerDeltaMeshRoutes(r, conf)

	t.Run("well-known", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/deltamesh", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}

		var payload DeltaMeshWellKnown
		if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}

		expectedBase := "http://127.0.0.1:4242"
		if payload.Node != expectedBase {
			t.Fatalf("expected node %s, got %s", expectedBase, payload.Node)
		}

		if payload.Endpoints["node"] != expectedBase+"/deltamesh/node" {
			t.Fatalf("unexpected node endpoint: %s", payload.Endpoints["node"])
		}
	})

	t.Run("node status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/deltamesh/node", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}

		var payload DeltaMeshNode
		if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}

		if payload.Implementation != "stegodon" {
			t.Fatalf("unexpected implementation: %s", payload.Implementation)
		}

		if payload.Node == "" {
			t.Fatalf("expected node identifier to be set")
		}
	})

	t.Run("catalog", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/deltamesh/repos", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}

		var payload DeltaMeshRepositoryCatalog
		if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}

		if len(payload.Repositories) != 0 {
			t.Fatalf("expected empty catalog, got %d entries", len(payload.Repositories))
		}
	})
}
