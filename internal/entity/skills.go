package entity

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// Skill grants (Thought-Processor #72). Identity grants only skills on this fixed allowlist;
// a model, lesson or GitHub text can never add one. Each skill gets its own run grant, so
// conversation, lesson.learn, pulse.run etc. never imply that any skill may run.
//
//	skill.discover                      Core may show the owner which registered skills exist
//	skill.run:<skill id>                Core may run exactly that skill, after the owner approves the request
var KnownSkills = map[string]string{
	"lesson-workspace.list-lessons": "list the lesson files in TASS's local, read-only lesson workspace (no code runs)",
	// Information junction (Thought-Processor #82 P2): a fixed, read-only Go handler the Core's
	// 20-second loop may use without a per-run approval once the owner grants it here.
	"junction.lesson-metadata": "let TASS's 20-second loop read one lesson's metadata (title, goal, checks) through a fixed read-only handler; no code runs, nothing is written",
}

const SkillDiscover = "skill.discover"

// SkillRunGrant is the grant a known entry needs. Junctions use their own grant name so that
// a junction grant never implies a skill run (and the reverse).
func SkillRunGrant(id string) string {
	if strings.HasPrefix(id, "junction.") {
		return "junction.read:" + strings.TrimPrefix(id, "junction.")
	}
	return "skill.run:" + id
}

var ErrUnknownSkill = errors.New("unknown skill: Identity only grants skills on its allowlist")

// SkillList is what the owner page shows.
func SkillList() []map[string]string {
	ids := make([]string, 0, len(KnownSkills))
	for id := range KnownSkills {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := []map[string]string{}
	for _, id := range ids {
		out = append(out, map[string]string{"id": id, "purpose": KnownSkills[id], "grant": SkillRunGrant(id)})
	}
	return out
}

// SetSkillGrant: enabling needs an unlocked entity and the owner's passphrase again (it widens
// what TASS may do). Revoking needs only the unlocked owner page: it can only take authority away,
// so it must be quick. Same entity ID, credential and other grants; atomic write; audit line.
func (s *Store) SetSkillGrant(passphrase, skill string, enable bool) (State, error) {
	if _, ok := KnownSkills[skill]; !ok {
		return State{}, ErrUnknownSkill
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, err := s.load()
	if err != nil {
		return State{}, fmt.Errorf("%w: %v", ErrNotUsable, err)
	}
	if e == nil {
		return State{}, ErrNoEntity
	}
	now := s.Now()
	if !now.Before(s.until) {
		return State{}, ErrLocked
	}
	if enable {
		wait := time.Duration(0)
		if s.fails >= 3 {
			wait = time.Duration(1<<min(s.fails-3, 8)) * time.Second
		}
		if now.Before(s.failedAt.Add(wait)) {
			return State{}, ErrThrottled
		}
		if !e.Verifier.Check(passphrase) {
			s.fails++
			s.failedAt = now
			return State{}, ErrBadPass
		}
		s.fails = 0
	}
	before := strings.Join(e.Grants, ",")
	run := SkillRunGrant(skill)
	if enable {
		add := []string{SkillDiscover, run}
		if strings.HasPrefix(skill, "junction.") {
			add = []string{run} // a junction is not a skill: no skill.discover
		}
		for _, g := range add {
			if !contains(e.Grants, g) {
				e.Grants = append(e.Grants, g)
			}
		}
	} else {
		kept := e.Grants[:0:0]
		for _, g := range e.Grants {
			if g != run {
				kept = append(kept, g)
			}
		}
		e.Grants = kept
	}
	if strings.Join(e.Grants, ",") != before {
		b, err := json.MarshalIndent(e, "", "  ")
		if err != nil {
			return State{}, err
		}
		if err := writeFile(s.path("entity.json"), b); err != nil {
			return State{}, err
		}
	}
	action, reason := "grants.skill.enable", "owner passphrase confirmed; skill run grant added"
	if !enable {
		action, reason = "grants.skill.revoke", "owner revoked the skill run grant"
	}
	d := Decision{ID: "dec_" + randHex(8), At: now.UTC(), Service: "owner", Action: action, Detail: run,
		Allow: true, Reason: reason, EntityID: e.EntityID}
	b, _ := json.Marshal(d)
	f, err := os.OpenFile(s.path("decisions.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return State{}, fmt.Errorf("skill grant saved but audit failed: %w", err)
	}
	_, err = f.Write(append(b, '\n'))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return State{}, fmt.Errorf("skill grant saved but audit failed: %w", err)
	}
	u := s.until
	return State{State: Unlocked, Entity: e.public(), UnlockedUntil: &u}, nil
}
