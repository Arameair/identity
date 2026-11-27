package entity

// mailbox.read (TASS #24, junction rev 5; audits 7224748979, 7225044922): Core may obtain a decision
// to read the ONE mailbox thread the OWNER mapped to this entity in mailbox-scopes.json (same
// owner-table pattern as memory-audiences.json). Identity computes the binding from the entity, that
// thread and the junction; a caller-supplied binding or thread is never trusted. Core can neither
// redeem nor check: the protected, redeem-only mailbox reader redeems BEFORE it reads (binding must
// match, so it can only read the owner-mapped thread) and CHECKs again for delivery.
//
// Delivery boundary: the reader's successful CHECK is the authorization point for one delivery.
// A revocation Identity commits before it refuses that delivery; one committed after it is ordered
// after that single delivery and refuses every later check, redemption and mint.
// Not a default grant; never implies github.mailbox.reply or memory.read.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

const (
	MailboxRead          = "mailbox.read"
	MailboxReaderService = "mailbox-reader"
	mailboxScopeFile     = "mailbox-scopes.json" // owner-managed: {"<entity_id>": "owner/repo#issue"}
	mailboxContract      = "tass-mailbox-read-v1"
)

var (
	MailboxThreadRe   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}/[A-Za-z0-9._-]{1,100}#[1-9][0-9]{0,8}$`)
	mailboxJunctionRe = regexp.MustCompile(`^tj_[A-Za-z0-9_]{1,64}$`)
)

func init() {
	BoundActions[MailboxRead] = true
	BoundRedeemers[MailboxRead] = MailboxReaderService
	KnownSkills[MailboxRead] = "let the entity READ the one team mailbox thread you mapped to it in mailbox-scopes.json, to answer your explicit request in Talk; each answer uses its own single-use approval, checked again just before it is shown; revoking stops further answers at once; nothing is posted"
	ownerGrants[MailboxRead] = MailboxRead
}

// MailboxBinding is the exact scope of one decision (length-prefixed, like MemoryBinding).
func MailboxBinding(entityID, thread, junctionID string) string {
	h := sha256.New()
	for _, p := range []string{mailboxContract, MailboxRead, entityID, thread, junctionID} {
		fmt.Fprintf(h, "%d:%s|", len(p), p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// MailboxThread is the owner's mapping for this entity ("" = none: nothing can be minted).
func (s *Store) MailboxThread(entityID string) string {
	b, err := os.ReadFile(s.path(mailboxScopeFile))
	if err != nil {
		return ""
	}
	var m map[string]string
	if json.Unmarshal(b, &m) != nil {
		return ""
	}
	if t := m[entityID]; MailboxThreadRe.MatchString(t) {
		return t
	}
	return ""
}

// MintMailbox: Identity derives thread and binding; the caller names only the junction.
func (s *Store) MintMailbox(service, entityID, junctionID string) (BoundDecision, string) {
	ent := entityID
	if st := s.State(); ent == "" && st.Entity != nil {
		ent = st.Entity.EntityID
	}
	thread := s.MailboxThread(ent)
	if thread == "" || !mailboxJunctionRe.MatchString(junctionID) {
		d := BoundDecision{Decision: Decision{ID: "dec_" + randHex(8), At: s.Now().UTC(), Service: service, Action: MailboxRead,
			Detail: trim(junctionID, 80), Reason: "no owner-mapped mailbox thread for this entity, or an invalid junction"}, Mode: "mint"}
		s.auditBound(d)
		return d, ""
	}
	return s.MintBound(service, MailboxRead, MailboxBinding(ent, thread, junctionID), thread+" "+junctionID, entityID), thread
}

// mailboxMintOK re-derives the scope inside MintBound, so no route can mint another thread.
func (s *Store) mailboxMintOK(binding, detail, entityID string) bool {
	ent := entityID
	if st := s.State(); ent == "" && st.Entity != nil {
		ent = st.Entity.EntityID
	}
	thread, junction, ok := strings.Cut(detail, " ")
	return ok && thread != "" && thread == s.MailboxThread(ent) && mailboxJunctionRe.MatchString(junction) &&
		binding == MailboxBinding(ent, thread, junction)
}
