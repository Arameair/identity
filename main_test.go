package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Arameair/identity/internal/publisher"
)

func TestClientIDChoice(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("IDENTITY_GITHUB_CLIENT_ID", "")
	old := publisher.GitHubAppClientID
	defer func() { publisher.GitHubAppClientID = old }()

	publisher.GitHubAppClientID = ""
	if cur, _ := clientID(dir); cur.ID != "" {
		t.Fatalf("no built-in app should mean none: %+v", cur)
	}

	publisher.GitHubAppClientID = "Iv23liBUILTIN0001"
	cur, def := clientID(dir)
	if cur.ID != "Iv23liBUILTIN0001" || cur.Kind != "built-in" || def != cur {
		t.Fatalf("built-in not used by default: %+v", cur)
	}

	os.WriteFile(filepath.Join(dir, "github-app.json"), []byte(`{"client_id":"Iv23liOWNAPP00001"}`), 0o600)
	cur, def = clientID(dir)
	if cur.ID != "Iv23liOWNAPP00001" || cur.Kind != "own" || def.Kind != "built-in" {
		t.Fatalf("own app from guide not preferred, or default lost: %+v %+v", cur, def)
	}

	t.Setenv("IDENTITY_GITHUB_CLIENT_ID", "Iv23liDEVELOPER01")
	if cur, _ := clientID(dir); cur.Kind != "developer" {
		t.Fatalf("developer override not applied: %+v", cur)
	}
}
