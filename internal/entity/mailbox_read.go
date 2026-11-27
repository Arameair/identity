package entity

// mailbox.read (TASS #24, junction rev 5 accepted in 7224477017): Core may read the ONE pinned team
// mailbox thread to answer an owner's explicit Talk request, and write the answer into the original
// conversation. Each delivery needs its own bound, single-use decision. Core redeems it while
// holding its Talk writer lock, immediately before the durable append.
//
// The redemption is the irreversible authorization boundary. RedeemBound re-checks UNLOCKED, the
// entity and this grant under the bound-table lock, so a revocation that commits first is refused.
// A revocation that commits after the redemption cannot recall that one authorized append.
//
// Not a default grant. It never implies github.mailbox.reply, memory.read or any skill, and posts nothing.

const (
	MailboxRead = "mailbox.read"
	// PinnedMailboxThread is the only thread a mailbox.read decision may name. A mint whose detail
	// names any other thread is refused by Identity, whatever binding Core computed.
	PinnedMailboxThread = "Arameair/TASS#24"
)

// coreRedeemed: bound actions whose effect Core itself performs (a local, durable append), so Core
// is their redeemer. Core keeps its normal routes; it is not a redeem-only service.
var coreRedeemed = map[string]bool{MailboxRead: true}

func init() {
	BoundActions[MailboxRead] = true
	SinglePointOfUse[MailboxRead] = true
	KnownSkills[MailboxRead] = "let the entity READ the one team mailbox thread (" + PinnedMailboxThread +
		") to answer your explicit request in Talk; each answer uses its own single-use approval, redeemed just before it is written; revoking stops further answers at once; nothing is posted"
	ownerGrants[MailboxRead] = MailboxRead
}

// mailboxThreadOK: a mailbox.read mint must name the pinned thread as the first field of its detail.
func mailboxThreadOK(detail string) bool {
	return detail == PinnedMailboxThread || len(detail) > len(PinnedMailboxThread) &&
		detail[:len(PinnedMailboxThread)+1] == PinnedMailboxThread+" "
}
