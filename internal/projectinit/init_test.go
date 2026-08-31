package projectinit

import (
	"context"
	"errors"
	"testing"

	"github.com/webxsid/pdg/internal/config"
	"github.com/webxsid/pdg/internal/protocol/atproto"
)

var errFakeMissing = errors.New("fake session missing")

type fakeSessions struct {
	did     string
	session atproto.Session
}

func (f fakeSessions) LoadSession(_ context.Context, did string) (atproto.Session, error) {
	if did != f.did {
		return atproto.Session{}, errFakeMissing
	}
	return f.session, nil
}

type fakePublisher struct {
	calls int
}

func (f *fakePublisher) CreateRecord(context.Context, atproto.Session, string, any) (string, error) {
	f.calls++
	return "at://did:plc:configured/site.standard.publication/3example", nil
}

func TestRunUsesConfiguredDIDAndPersistsPublication(t *testing.T) {
	publisher := &fakePublisher{}
	service := &Service{
		ConfigPath: t.TempDir() + "/pdg.yaml",
		Sessions: fakeSessions{
			did:     "did:plc:configured",
			session: atproto.Session{},
		},
		XRPC: publisher,
	}
	got, err := service.Run(context.Background(), Options{
		SiteURL:               "https://example.com",
		ATProtoEnabled:        true,
		ATProtoDID:            "did:plc:configured",
		StandardSiteEnabled:   true,
		ConfigureIntegrations: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if publisher.calls != 1 {
		t.Fatalf("CreateRecord calls = %d, want 1", publisher.calls)
	}
	if got.ATProto.StandardSite.Publication == "" {
		t.Fatal("publication URI was not persisted")
	}
	loaded, err := config.LoadConfig(service.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ATProto.Identity != "did:plc:configured" {
		t.Errorf("identity = %q, want configured DID", loaded.ATProto.Identity)
	}
}
