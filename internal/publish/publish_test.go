package publish

import (
	"context"
	"testing"
	"time"

	"github.com/webxsid/pdg/internal/protocol/atproto"
	"github.com/webxsid/pdg/internal/state"
)

type fakeRecords struct {
	created int
	updated int
}

func (f *fakeRecords) CreateRecord(context.Context, atproto.Session, string, any) (string, error) {
	f.created++
	return "at://did:plc:test/site.standard.document/abc", nil
}

func (f *fakeRecords) UpdateRecord(context.Context, atproto.Session, string, string, any) (string, error) {
	f.updated++
	return "bafycid", nil
}

func testProject(target state.TargetState) state.ProjectState {
	project := state.New()
	project.Integrations["standard-site"] = state.IntegrationState{Publication: &state.PublicationState{URI: "at://did:plc:test/site.standard.publication/pub"}}
	project.Documents["/hello"] = &state.DocumentState{
		Path: "/hello", Fingerprint: "sha256:new", Metadata: state.DocumentMetadata{
			Title: "Hello", TextContent: "Text", PublishedAt: timePtr(),
		}, Targets: map[state.PublicationTarget]state.TargetState{state.TargetStandardSite: target},
	}
	return project
}

func timePtr() *time.Time {
	value := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return &value
}

func TestStandardSiteCreatesAndPersists(t *testing.T) {
	project := testProject(state.TargetState{Selected: true})
	client := &fakeRecords{}
	persisted := 0
	summary := StandardSite(context.Background(), &project, atproto.Session{Identity: atproto.Identity{DID: "did:plc:test"}}, client, func(state.ProjectState) error { persisted++; return nil })
	if len(summary.Errors) != 0 || client.created != 1 || persisted != 1 {
		t.Fatalf("summary=%+v created=%d persisted=%d", summary, client.created, persisted)
	}
	target := project.Documents["/hello"].Targets[state.TargetStandardSite]
	if target.URI == "" || target.PublishedFingerprint != "sha256:new" {
		t.Fatalf("target = %+v", target)
	}
}

func TestStandardSiteNoopDoesNotMutate(t *testing.T) {
	project := testProject(state.TargetState{Selected: true, URI: "at://did:plc:test/site.standard.document/abc", PublishedFingerprint: "sha256:new"})
	client := &fakeRecords{}
	persisted := 0
	summary := StandardSite(context.Background(), &project, atproto.Session{Identity: atproto.Identity{DID: "did:plc:test"}}, client, func(state.ProjectState) error { persisted++; return nil })
	if len(summary.Errors) != 0 || len(summary.Results) != 1 || summary.Results[0].Action != ActionNoop || client.created != 0 || client.updated != 0 || persisted != 0 {
		t.Fatalf("summary=%+v created=%d updated=%d persisted=%d", summary, client.created, client.updated, persisted)
	}
}

func TestStandardSiteUpdatesExistingRecord(t *testing.T) {
	project := testProject(state.TargetState{Selected: true, URI: "at://did:plc:test/site.standard.document/abc", PublishedFingerprint: "sha256:old"})
	client := &fakeRecords{}
	summary := StandardSite(context.Background(), &project, atproto.Session{Identity: atproto.Identity{DID: "did:plc:test"}}, client, func(state.ProjectState) error { return nil })
	if len(summary.Errors) != 0 || client.updated != 1 || client.created != 0 {
		t.Fatalf("summary=%+v created=%d updated=%d", summary, client.created, client.updated)
	}
	target := project.Documents["/hello"].Targets[state.TargetStandardSite]
	if target.URI != "at://did:plc:test/site.standard.document/abc" || target.CID != "bafycid" || target.PublishedFingerprint != "sha256:new" {
		t.Fatalf("target = %+v", target)
	}
}
