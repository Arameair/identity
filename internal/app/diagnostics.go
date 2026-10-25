package app

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/Arameair/identity/internal/logbook"
	"github.com/Arameair/identity/internal/profile"
	"github.com/Arameair/identity/internal/redact"
)

// Diagnostics builds the plain-text Copy Diagnostics report. Everything is
// passed through redact.String at the end, so even a secret that slipped
// into a log detail cannot leave in the report.
func (a *App) Diagnostics(requestID string) string {
	now := time.Now()
	gv := a.GH.View()
	set := a.Store.Settings()
	pr := a.Watch.Current()
	ls := a.Log.Stats()
	var b strings.Builder
	p := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }

	p("=== IDENTITY DIAGNOSTICS — Prototype B (Claude) ===")
	p("Paste-safe: credentials, session links, one-time codes, cookies and tokens are removed.")
	p("Generated: %s UTC | %s local | request %s", now.UTC().Format(time.RFC3339), now.Format("2006-01-02 15:04:05 MST -07:00"), requestID)
	p("")
	p("[build]")
	p("app=identity version=%s protocol=%s", a.Build.Version, Protocol)
	p("repo=Arameair/identity branch=%s", a.Build.Branch)
	p("commit=%s modified=%s commit_time=%s", a.Build.Revision, a.Build.Modified, a.Build.VCSTime)
	p("go=%s os=%s arch=%s", a.Build.GoVersion, runtime.GOOS, runtime.GOARCH)
	if a.Build.Revision == "UNKNOWN" {
		p("note: commit is UNKNOWN — this binary was not built from a git checkout, so its source cannot be verified")
	}
	p("")
	p("[process]")
	p("status=RUNNING pid=%d run=%s bind=127.0.0.1 port=%d uptime=%s", os.Getpid(), a.RunID, a.Port, time.Since(a.Started).Round(time.Second))
	p("started=%s UTC", a.Started.UTC().Format(time.RFC3339))
	p("profile=%s system_id=%s profile_created=%s", profile.DisplayPath(a.Store.Root), a.Store.Profile.SystemID, a.Store.Profile.CreatedAt.Format(time.RFC3339))
	p("open_windows(sessions)=%d", a.Sess.Count())
	p("endpoints: GET /api/health (public, self-reported); everything else requires this run's operator session + CSRF")
	if set.SetupCompletedAt != nil {
		p("setup=COMPLETED at %s", set.SetupCompletedAt.Format(time.RFC3339))
	} else {
		p("setup=IN PROGRESS (welcome flow shown)")
	}
	p("")
	p("[github]")
	p("mode=%s status=%s github_app=%s kind=%s source=%s", gv.Mode, strings.ToUpper(gv.Status), yesNo(gv.ClientIDSet, "set (public client id "+gv.ClientIDHint+")", "NONE IN THIS BUILD"), orDash(gv.ClientIDKind), orDash(gv.ClientIDSource))
	if gv.Account != nil {
		p("verified_account=%s id=%d verified_at=%s (confirmed by GitHub API this session)", gv.Account.Login, gv.Account.ID, gv.Account.VerifiedAt.Format(time.RFC3339))
		if gv.TokenExpiresAt != nil {
			p("session_token=memory only, expires %s", gv.TokenExpiresAt.Format(time.RFC3339))
		}
	} else if set.LastAccount != nil {
		p("verified_account=NONE this session; last verified %s (id %d, %s) at %s — history only, not signed in now", set.LastAccount.Login, set.LastAccount.ID, set.LastAccount.Mode, set.LastAccount.VerifiedAt.Format(time.RFC3339))
	} else {
		p("verified_account=NONE")
	}
	if gv.Message != "" {
		p("last_message=%s", gv.Message)
	}
	if gv.LastCheckedAt != nil {
		p("waiting: last_checked=%s (%s ago) github_said=%s", gv.LastCheckedAt.Format(time.RFC3339), time.Since(*gv.LastCheckedAt).Round(time.Second), orDash(gv.PollNote))
	} else if gv.Status == "waiting" {
		p("waiting: GitHub not asked yet (first check comes ~5 s after the code is issued)")
	}
	if gv.Publisher != nil {
		p("publisher_app_installations=%s count=%d %s", gv.Publisher.InstallationsResult, len(gv.Publisher.Installations), gv.Publisher.InstallationsDetail)
		for _, i := range gv.Publisher.Installations {
			p("  installation id=%d app=%s on=%s repos=%s", i.ID, i.AppSlug, i.Account, i.RepositorySelection)
		}
		for _, r := range gv.Publisher.Repos {
			p("  repo %s: %s (http %d, %s)", r.Repo, r.Result, r.HTTP, r.CheckedAt.Format(time.RFC3339))
		}
	}
	p("publisher=%s (name embedded in build; not a signed release)", Publisher)
	p("")
	p("[thought processor — read-only discovery]")
	p("address=%s state=%s freshness=%s", orDash(pr.Address), pr.State, pr.Fresh)
	if !pr.CheckedAt.IsZero() {
		p("checked=%s elapsed_ms=%d http=%d", pr.CheckedAt.Format(time.RFC3339), pr.ElapsedMS, pr.HTTPStatus)
	}
	if pr.ClaimedApp != "" {
		p("self_reported: app=%s version=%s protocol=%s (UNTRUSTED CLAIMS)", pr.ClaimedApp, pr.ClaimedVersion, pr.ClaimedProtocol)
	}
	p("note=%s", pr.Note)
	p("authentication=NONE (no Identity↔Processor contract exists); Identity never reads Processor's operator code")
	p("")
	p("[features — states only from evidence]")
	for _, c := range a.Feats.Cards(gv) {
		p("%s (%s): availability=%s | request=%s | grant=%s", c.Name, c.Repo, c.Availability, c.Request, c.Grant)
	}
	p("local_drafts_total=%d (all NOT SUBMITTED)", len(a.Feats.All()))
	if e := a.Feats.LoadError(); e != "" {
		p("drafts_error=%s", e)
	}
	p("")
	p("[log]")
	p("file=%s in_memory=%d loaded_at_start=%d unreadable_lines=%d write_errors=%d rotations=%d last_id=%d",
		profile.DisplayPath(ls.Path), ls.InMemory, ls.LoadedFromDisk, ls.UnreadableLines, ls.WriteErrors, ls.Rotations, ls.LastID)
	if ls.LastWriteError != "" {
		p("last_write_error=%s", ls.LastWriteError)
	}
	p("")
	p("[latest problems (warn/error), newest last]")
	probs := a.Log.Recent(10, true)
	if len(probs) == 0 {
		p("(none recorded)")
	}
	for _, e := range probs {
		p("%s", logbook.Line(e))
	}
	p("")
	p("[recent activity, newest last — up to 60]")
	for _, e := range a.Log.Recent(60, false) {
		p("%s [%s]", logbook.Line(e), e.RunID)
	}
	p("")
	p("[not implemented — never shown as working]")
	for _, s := range NotImplemented {
		p("- %s", s)
	}
	p("")
	p("[to reproduce]")
	p("1. Build: go build -o identity-b.exe .  (from a checkout of branch %s, commit above)", a.Build.Branch)
	p("2. Run:   .\\identity-b.exe -port %d -data <profile folder>", a.Port)
	p("3. Thought Processor separately on its own port; set its address in Identity > Manage > Thought Processor")
	p("=== END ===")
	return redact.String(b.String())
}

func yesNo(c bool, y, n string) string {
	if c {
		return y
	}
	return n
}
