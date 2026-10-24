package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeGH struct {
	user              string
	repo              RepoInfo
	err               error
	checks, repoReads int
}

func (g *fakeGH) User(ctx context.Context) (string, error) { g.checks++; return g.user, g.err }
func (g *fakeGH) Repository(ctx context.Context, r string) (RepoInfo, error) {
	g.repoReads++
	return g.repo, g.err
}
func testApp(t *testing.T, g githubOps) *App {
	t.Helper()
	a, e := openApp(t.TempDir(), "alpha", "sha", "7796", g)
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func TestIdentityAddArchiveRestoreAndRepoNeverDeleted(t *testing.T) {
	gh := &fakeGH{user: "octocat", repo: RepoInfo{FullName: "octocat/test-private", Private: true, URL: "https://github.com/octocat/test-private"}}
	a := testApp(t, gh)
	if _, e := a.connect(); e != nil {
		t.Fatal(e)
	}
	c, e := a.add("TASS", "ai_companion")
	if e != nil {
		t.Fatal(e)
	}
	if e = a.linkRepo(c.ID, "octocat/test-private"); e != nil {
		t.Fatal(e)
	}
	if e = a.archive(c.ID, false); e != nil {
		t.Fatal(e)
	}
	items := a.snapshot().Cards
	if items[0].Repo != "octocat/test-private" || items[0].ArchivedAt == nil {
		t.Fatalf("archival lost link or didn't archive: %+v", items)
	}
	if gh.repoReads != 1 {
		t.Fatal("repository should be READ once for verification; never modified")
	}
	if e = a.archive(c.ID, true); e != nil {
		t.Fatal(e)
	}
	if a.snapshot().Cards[0].ID != c.ID {
		t.Fatal("restore must preserve ID")
	}
	data := a.dataDir
	b, e := openApp(data, "alpha", "sha", "7796", gh)
	if e != nil {
		t.Fatal(e)
	}
	if b.snapshot().Cards[0].Repo != "octocat/test-private" || b.snapshot().Cards[0].ArchivedAt != nil {
		t.Fatal("restart lost repo link or restore state")
	}
}
func TestDisabledOrUnknownIdentityCannotBeRemoved(t *testing.T) {
	a := testApp(t, nil)
	if e := a.archive("garbage", false); e == nil {
		t.Fatal("accepted unknown identity")
	}
}
func TestConnectedAccountMustMatchCLI(t *testing.T) {
	g := &fakeGH{user: "first"}
	a := testApp(t, g)
	if _, e := a.connect(); e != nil {
		t.Fatal(e)
	}
	g.user = "second"
	if _, e := a.boundLogin(context.Background()); e == nil {
		t.Fatal("linked account mismatch accepted")
	}
}
func TestConnectRequiresRealAuthenticatedAccount(t *testing.T) {
	g := &fakeGH{err: errors.New("not authenticated")}
	a := testApp(t, g)
	if _, e := a.connect(); e == nil {
		t.Fatal("connected without auth")
	}
	if a.snapshot().GitHub != nil {
		t.Fatal("bad auth saved")
	}
}
func TestRepoMustBePrivateAndVerified(t *testing.T) {
	g := &fakeGH{user: "octocat", repo: RepoInfo{FullName: "octocat/public", URL: "https://github.com/octocat/public"}}
	a := testApp(t, g)
	_, _ = a.connect()
	c, _ := a.add("one", "project")
	if e := a.linkRepo(c.ID, "octocat/public"); e == nil {
		t.Fatal("public repository accepted")
	}
}
func TestDuplicateAndIsolation(t *testing.T) {
	a := testApp(t, nil)
	c, e := a.add("TASS", "ai_companion")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.add("tass", "ai_companion"); e == nil {
		t.Fatal("duplicate accepted")
	}
	b, e := a.add("Business", "business")
	if e != nil {
		t.Fatal(e)
	}
	if b.ID == c.ID {
		t.Fatal("ids collided")
	}
	_ = a.archive(c.ID, false)
	if a.snapshot().Cards[1].ArchivedAt != nil {
		t.Fatal("cross-card removal")
	}
}
func TestNoRepoDeletionCallImplemented(t *testing.T) {
	b, e := os.ReadFile("main.go")
	if e != nil {
		t.Fatal(e)
	}
	source := string(b)
	for _, bad := range []string{"DELETE /repos/", "gh repo delete", "api repos/ -X DELETE"} {
		if strings.Contains(source, bad) {
			t.Fatal("deletion operation introduced")
		}
	}
}
func TestCorruptStoreFailClosed(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "identity.json"), []byte("not-json"), 0600)
	if _, e := openApp(d, "alpha", "sha", "7796", nil); e == nil {
		t.Fatal("corruption unnoticed")
	}
}
func TestGitHubEnvironmentStrip(t *testing.T) {
	env := cleanEnv([]string{"PATH=/bin", "GH_TOKEN=secret", "GITHUB_TOKEN=other", "HOME=/x"})
	str := strings.Join(env, "\n")
	if strings.Contains(str, "secret") || strings.Contains(str, "other") || !strings.Contains(str, "GH_HOST=github.com") {
		t.Fatal("token leaked")
	}
}

func TestDataFolderLock(t *testing.T) {
	d := t.TempDir()
	release, err := acquireLock(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acquireLock(d); err == nil {
		t.Fatal("second instance obtained the same lock")
	}
	release()
	again, err := acquireLock(d)
	if err != nil {
		t.Fatal(err)
	}
	again()
}
