package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

const (
	EnhanceFastAIWorker = "enhance.fast_ai_worker.send"
	EnhanceTeamMember   = "enhance.team_member.send"
	enhancementRecipientsFile = "enhancement-recipients.json"
)

type EnhancementScope struct {
	SparkID       string `json:"spark_id"`
	RequestID     string `json:"request_id"`
	PayloadSHA256 string `json:"payload_sha256"`
	Recipient     string `json:"recipient"`
	PolicyVersion string `json:"policy_version"`
	FenceEpoch    uint64 `json:"fence_epoch"`
}

var (
	enhancementID = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)
	enhancementDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)
	enhancementRecipientID = regexp.MustCompile(`^[A-Za-z0-9_.:/-]{1,256}$`)
)

func IsEnhancementAction(action string) bool {
	return action == EnhanceFastAIWorker || action == EnhanceTeamMember
}

func enhancementBinding(action, entityID string, sc EnhancementScope) string {
	h := sha256.New()
	for _, part := range []string{"tass-enhancement-v2", action, entityID, sc.SparkID, sc.RequestID, sc.PayloadSHA256, sc.Recipient, sc.PolicyVersion, fmt.Sprint(sc.FenceEpoch)} {
		fmt.Fprintf(h, "%d:%s|", len(part), part)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// The recipient table lives under Identity's private data directory. A missing, malformed, or
// mismatched pin denies. The Core service request can never set or override a pin.
func (s *Store) enhancementRecipientAllowed(action string, scope EnhancementScope) bool {
	if !enhancementRecipientID.MatchString(scope.Recipient) || !enhancementID.MatchString(scope.PolicyVersion) || scope.FenceEpoch == 0 {
		return false
	}
	raw, err := os.ReadFile(s.path(enhancementRecipientsFile))
	if err != nil {
		return false
	}
	var pins map[string]struct {
		Recipient string `json:"recipient"`
		PolicyVersion string `json:"policy_version"`
		FenceEpoch uint64 `json:"fence_epoch"`
	}
	if json.Unmarshal(raw, &pins) != nil {
		return false
	}
	pin := pins[action]
	return pin.Recipient == scope.Recipient && pin.PolicyVersion == scope.PolicyVersion && pin.FenceEpoch == scope.FenceEpoch
}

// MintEnhancement validates the cleartext scope and computes the binding inside Identity.
// The dedicated owner grant is checked by the existing bound decision mechanism.
func (s *Store) MintEnhancement(service, action, entityID string, scope *EnhancementScope) BoundDecision {
	deny := func(reason string) BoundDecision {
		d := BoundDecision{Decision: Decision{ID: "dec_" + randHex(8), At: s.Now().UTC(), Service: service, Action: action,
			Reason: reason}, Mode: "mint"}
		s.auditBound(d)
		return d
	}
	if service != MintService || !IsEnhancementAction(action) || !enhancementID.MatchString(entityID) ||
		scope == nil || !enhancementID.MatchString(scope.SparkID) || !enhancementID.MatchString(scope.RequestID) ||
		!enhancementDigest.MatchString(scope.PayloadSHA256) {
		return deny("invalid or unauthorized enhancement scope")
	}
	s.enhancementMu.Lock()
	defer s.enhancementMu.Unlock()
	if !s.enhancementRecipientAllowed(action, *scope) {
		return deny("enhancement recipient is not pinned by Identity")
	}
	return s.mintBound(service, action, enhancementBinding(action, entityID, *scope), "", entityID, scope)
}
