package features

// Feature contract, version 1 ("thought.feature/1").
//
// The smallest description Thought Processor needs to install any feature
// without feature-specific code: where approved releases come from, how the
// file for this computer is named, how to check it, and (for later) how the
// feature runs. Identity is the authority that hands it out, and only to
// people allowed to see the feature (private features stay hidden).
//
// Honest limit: this contract is embedded in this Identity build. It is not
// signed by the publisher, and a release checksum proves integrity, not who
// published it. See ApprovedSource for what IS enforced.

import (
	"fmt"
	"regexp"
	"strings"
)

const ContractSchema = "thought.feature/1"

// Visibility.
const (
	Public  = "public"  // shown to everyone
	Private = "private" // shown only when the person's verified account passes the feature's access rule
)

// ApprovedSource says which releases are acceptable for a feature. Identity
// refuses (UNAPPROVED_SOURCE) any release that doesn't match; Processor checks
// the same rules again against what Identity reports.
type ApprovedSource struct {
	Kind      string `json:"kind"`       // "github-release"
	Repo      string `json:"repo"`       // e.g. Arameair/Thought-engine
	TagPrefix string `json:"tag_prefix"` // e.g. engine-v
	// AssetTemplate names the file for one platform: {os}, {arch}, {exe}
	// (".exe" on Windows, "" elsewhere).
	AssetTemplate string `json:"asset_template"`
	Checksums     string `json:"checksums"` // file in the same release: "<sha256>  <asset>" lines
	// TrustedUploaders: GitHub accounts allowed to have uploaded the asset and
	// the checksum file. "github-actions[bot]" = built and published by the
	// repository's own release workflow, not uploaded by hand.
	TrustedUploaders []string `json:"trusted_uploaders"`
	AllowPrerelease  bool     `json:"allow_prerelease"`
}

type Install struct {
	Kind     string `json:"kind"`      // "single-executable"
	MaxBytes int64  `json:"max_bytes"` // refuse anything larger
}

// Run is informational in P0: Processor does not start features yet.
type Run struct {
	DefaultPort int               `json:"default_port,omitempty"`
	Health      string            `json:"health,omitempty"`
	Args        []string          `json:"args,omitempty"` // {port}, {data}
	Env         map[string]string `json:"env,omitempty"`  // {port}, {data}
}

type Contract struct {
	Schema      string          `json:"schema"`
	FeatureID   string          `json:"feature_id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Publisher   string          `json:"publisher"`
	Visibility  string          `json:"visibility"`
	Kind        string          `json:"kind"` // "interface" (Processor itself) | "service"
	Source      *ApprovedSource `json:"source,omitempty"`
	Install     *Install        `json:"install,omitempty"`
	Run         *Run            `json:"run,omitempty"`
	// Grants are separate from installing; listed so Processor can explain
	// what a feature would later ask for. Nothing here is granted.
	WouldAskFor []string `json:"would_ask_for,omitempty"`
}

var platformRE = regexp.MustCompile(`^(windows|linux|darwin)-(amd64|arm64)$`)

// ValidPlatform reports whether p looks like windows-amd64.
func ValidPlatform(p string) bool { return platformRE.MatchString(p) }

// AssetName returns the release file name for platform, e.g.
// thought-engine-windows-amd64.exe.
func (s ApprovedSource) AssetName(platform string) (string, error) {
	if !platformRE.MatchString(platform) {
		return "", fmt.Errorf("platform must look like windows-amd64")
	}
	parts := strings.SplitN(platform, "-", 2)
	exe := ""
	if parts[0] == "windows" {
		exe = ".exe"
	}
	r := strings.NewReplacer("{os}", parts[0], "{arch}", parts[1], "{exe}", exe)
	return r.Replace(s.AssetTemplate), nil
}

// Trusted reports whether login is one of the trusted uploaders.
func (s ApprovedSource) Trusted(login string) bool {
	for _, u := range s.TrustedUploaders {
		if strings.EqualFold(u, login) {
			return true
		}
	}
	return false
}

// ContractFor returns the contract for a catalog feature.
func ContractFor(f Feature) Contract {
	c := Contract{Schema: ContractSchema, FeatureID: f.ID, Name: f.Name, Description: f.Description,
		Publisher: "Arameair", Visibility: f.Visibility, Kind: "service"}
	switch f.ID {
	case "thought-processor":
		c.Kind = "interface"
	case "thought-engine":
		c.Source = &ApprovedSource{Kind: "github-release", Repo: f.Repo, TagPrefix: "engine-v",
			AssetTemplate: "thought-engine-{os}-{arch}{exe}", Checksums: "SHA256SUMS",
			TrustedUploaders: []string{"github-actions[bot]"}}
		c.Install = &Install{Kind: "single-executable", MaxBytes: 200 << 20}
		c.Run = &Run{DefaultPort: 7798, Health: "/api/status", Args: []string{"-data", "{data}"}, Env: map[string]string{"PORT": "{port}"}}
		c.WouldAskFor = []string{"observe: read events from a source you choose", "think: send text to its AI worker"}
	}
	return c
}
