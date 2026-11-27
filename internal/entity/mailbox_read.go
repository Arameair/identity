package entity

// mailbox.read (TASS #24, junction rev 5 accepted in 7224477017; audit 7224748979): Core may obtain
// a decision to read the ONE pinned team mailbox thread for an owner's explicit Talk request. Core
// can neither redeem it nor read the mailbox: the protected mailbox reader (its own service key,
// redeem-only like the memory gate) redeems it BEFORE it reads anything, then CHECKs it again
// (same rules, not consumed) when Core is about to write the answer, so a revocation committed
// first refuses delivery. Not a default grant; never implies github.mailbox.reply or memory.read.

const (
	MailboxRead = "mailbox.read"
	// MailboxReaderService is the protected reader's service name in the service-key file.
	MailboxReaderService = "mailbox-reader"
	// PinnedMailboxThread is the only thread a mailbox.read decision may name.
	PinnedMailboxThread = "Arameair/TASS#24"
)

func init() {
	BoundActions[MailboxRead] = true
	BoundRedeemers[MailboxRead] = MailboxReaderService
	KnownSkills[MailboxRead] = "let the entity READ the one team mailbox thread (" + PinnedMailboxThread +
		") to answer your explicit request in Talk; each answer uses its own single-use approval, checked again just before it is written; revoking stops further answers at once; nothing is posted"
	ownerGrants[MailboxRead] = MailboxRead
}

// mailboxThreadOK: a mailbox.read mint must name the pinned thread as the first field of its detail.
func mailboxThreadOK(detail string) bool {
	return detail == PinnedMailboxThread || len(detail) > len(PinnedMailboxThread) &&
		detail[:len(PinnedMailboxThread)+1] == PinnedMailboxThread+" "
}
