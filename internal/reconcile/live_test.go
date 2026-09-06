package reconcile

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/webxsid/pdg/internal/config"
	"github.com/webxsid/pdg/internal/state"
)

func TestVerifyStandardSiteLive(t *testing.T) {
	const publication = "at://did:plc:test/site.standard.publication/pub"
	const document = "at://did:plc:test/site.standard.document/doc"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/site.standard.publication":
			_, _ = w.Write([]byte(publication + "\n"))
		case "/hello":
			_, _ = w.Write([]byte(`<html><head><link rel="site.standard.document" href="` + document + `"></head><body></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	project := Project{
		Config: config.Config{Site: config.SiteConfig{URL: server.URL}, Integrations: config.IntegrationConfig{StandardSite: config.StandardSiteIntegration{Enabled: true, Identity: "did:plc:test"}}},
		State:  state.ProjectState{Version: 1, Integrations: map[string]state.IntegrationState{"standard-site": {Publication: &state.PublicationState{URI: publication}}}, Documents: map[string]*state.DocumentState{"/hello": {Path: "/hello", CanonicalURL: server.URL + "/hello", Targets: map[state.PublicationTarget]state.TargetState{state.TargetStandardSite: {URI: document}}}}},
	}
	findings := VerifyStandardSiteLive(t.Context(), project, server.Client())
	for _, finding := range findings {
		if finding.Status != OK {
			t.Fatalf("finding = %+v", finding)
		}
	}
	if len(findings) != 2 {
		t.Fatalf("findings = %d, want 2", len(findings))
	}
}
