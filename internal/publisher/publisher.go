// Package publisher holds what this build says about its publisher.
//
// GitHubAppClientID is the PUBLIC Client ID of the publisher's registered
// GitHub App (device flow enabled). It is not a secret: GitHub designed it to
// ship inside apps. The publisher fills it in ONCE (here, or at build time with
//
//	-ldflags "-X github.com/Arameair/identity/internal/publisher.GitHubAppClientID=Iv23li..."
//
// ) and every copy of Identity built from that source lets people just press
// Connect GitHub. No end user ever registers an app. See docs/PUBLISHER_GITHUB_APP.md.
package publisher

const Name = "Arameair"

// GitHubAppClientID — Arameair's GitHub App (public Client ID, added 2026-10-25).
var GitHubAppClientID = "Iv23livvYZb0CafGVssP"
