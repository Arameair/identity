package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Arameair/identity/internal/entity"
)

func TestEnhancementBoundAuthorization(t *testing.T) {
	dir := t.TempDir()
	st, err := entity.Open(dir, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	h := routes(st, map[string]string{engineKey: "engine", coreKey: "core", "fixture-sender-key": "enhancement-sender"}, "page-secret")
	owner := map[string]string{"X-Identity-Page": "page-secret"}
	const passphrase = "correct horse battery"
	status, _ := req(t, h, "POST", "127.0.0.1:8796", "/v1/entity", owner, map[string]string{"name": "TASS", "passphrase": passphrase})
	if status != 200 && status != 201 {
		t.Fatalf("create entity: status %d", status)
	}
	id := st.State().Entity.EntityID
	call := func(path, key string, body any) map[string]any {
		t.Helper()
		_, out := req(t, h, "POST", "identity:8796", path, map[string]string{"Authorization": "Bearer " + key}, body)
		return out
	}
	want := func(label string, d map[string]any, allow bool) {
		t.Helper()
		if d["allow"] != allow {
			t.Fatalf("%s: %v", label, d)
		}
	}
	grant := func(action string, revoke bool) {
		t.Helper()
		code, _ := req(t, h, "POST", "127.0.0.1:8796", "/v1/grants/skill", owner,
			map[string]any{"skill": action, "passphrase": passphrase, "revoke": revoke})
		if code != 200 {
			t.Fatalf("grant %s: status %d", action, code)
		}
	}
	pin := func(fast string) {
		t.Helper()
		b, err := json.Marshal(map[string]map[string]any{
			entity.EnhanceFastAIWorker: {"recipient": fast, "policy_version":"v1", "fence_epoch": uint64(1)},
			entity.EnhanceTeamMember: {"recipient":"fixture://team", "policy_version":"v1", "fence_epoch": uint64(1)},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "enhancement-recipients.json"), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mint := func(key, action, entityID string, scope entity.EnhancementScope) map[string]any {
		return call("/v1/authorize", key, map[string]any{"action": action, "entity_id": entityID, "enhancement_scope": scope})
	}
	redeem := func(key, action string, d map[string]any, digest any, check bool) map[string]any {
		return call("/v1/decisions/redeem", key,
			map[string]any{"action": action, "decision_id": d["decision_id"], "binding": digest, "check": check})
	}
	fast, team := entity.EnhanceFastAIWorker, entity.EnhanceTeamMember
	scope := entity.EnhancementScope{SparkID: "spark-a", RequestID: "te_a", PayloadSHA256: strings.Repeat("a", 64), Recipient: "fixture://fast", PolicyVersion: "v1", FenceEpoch: 1}
	want("no pin or grant", mint(coreKey, fast, id, scope), false)
	pin(scope.Recipient)
	want("pin alone", mint(coreKey, fast, id, scope), false)
	grant(fast, false)
	want("engine cannot mint", mint(engineKey, fast, id, scope), false)
	want("wrong entity", mint(coreKey, fast, "other", scope), false)
	want("opaque binding rejected", call("/v1/authorize", coreKey, map[string]string{"action": fast, "binding": bindA}), false)
	changed := scope
	changed.Recipient = "fixture://team"
	want("cross-recipient", mint(coreKey, fast, id, changed), false)
	want("Team grant independent", mint(coreKey, team, id, changed), false)

	d := mint(coreKey, fast, id, scope)
	want("scoped mint", d, true)
	if d["binding"] == nil {
		t.Fatal("Identity did not return the computed binding")
	}
	for _, mutate := range []func(*entity.EnhancementScope){
		func(s *entity.EnhancementScope) { s.SparkID = "spark-b" },
		func(s *entity.EnhancementScope) { s.RequestID = "te_b" },
		func(s *entity.EnhancementScope) { s.PayloadSHA256 = strings.Repeat("b", 64) },
	} {
		other := scope
		mutate(&other)
		o := mint(coreKey, fast, id, other)
		want("other exact tuple", o, true)
		want("changed tuple cannot redeem", redeem("fixture-sender-key", fast, d, o["binding"], false), false)
	}
	want("wrong action cannot redeem", redeem("fixture-sender-key", team, d, d["binding"], false), false)
	want("Core cannot redeem", redeem(coreKey, fast, d, d["binding"], false), false)
	want("Engine cannot redeem", redeem(engineKey, fast, d, d["binding"], false), false)
	want("Sender redeems once", redeem("fixture-sender-key", fast, d, d["binding"], false), true)
	want("replay denied", redeem("fixture-sender-key", fast, d, d["binding"], false), false)
	want("check cannot bypass consumption", redeem("fixture-sender-key", fast, d, d["binding"], true), false)

	pending := mint(coreKey, fast, id, scope)
	want("pending mint", pending, true)
	grant(fast, true)
	want("revoked mint denied", mint(coreKey, fast, id, scope), false)
	want("revoked decision denied", redeem("fixture-sender-key", fast, pending, pending["binding"], false), false)
	grant(fast, false)
	want("grant re-enable does not revive a decision", redeem("fixture-sender-key", fast, pending, pending["binding"], false), false)
	grant(team, false)
	want("Team uses its own pin and grant", mint(coreKey, team, id, changed), true)
	pending = mint(coreKey, fast, id, scope)
	want("new mint", pending, true)
	audit := filepath.Join(dir, "decisions.jsonl")
	if err := os.Remove(audit); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(audit, 0o700); err != nil {
		t.Fatal(err)
	}
	want("mint audit failure denied", mint(coreKey, fast, id, scope), false)
	want("redeem audit failure denied", redeem("fixture-sender-key", fast, pending, pending["binding"], false), false)
	if err := os.Remove(audit); err != nil {
		t.Fatal(err)
	}
	want("audit failure consumes decision", redeem("fixture-sender-key", fast, pending, pending["binding"], false), false)
	pending = mint(coreKey, fast, id, scope)
	want("mint after audit recovery", pending, true)
	for _, change := range []func(*entity.EnhancementScope){
		func(s *entity.EnhancementScope){ s.PolicyVersion = "revoked" },
		func(s *entity.EnhancementScope){ s.FenceEpoch++ },
	} {
		future := scope
		change(&future)
		want("wrong policy or fence cannot mint", mint(coreKey, fast, id, future), false)
		key := filepath.Join(dir, "enhancement-recipients.json")
		data, err := json.Marshal(map[string]any{fast:map[string]any{
			"recipient":future.Recipient,"policy_version":future.PolicyVersion,"fence_epoch":future.FenceEpoch,
		}})
		if err != nil { t.Fatal(err) }
		if err := os.WriteFile(key, data, 0o600); err != nil { t.Fatal(err) }
		want("stale decision cannot redeem", redeem("fixture-sender-key", fast, pending, pending["binding"], false), false)
		pin(scope.Recipient)
	}
	pin("fixture://replacement")
	want("recipient rotation invalidates pending decision", redeem("fixture-sender-key", fast, pending, pending["binding"], false), false)
}
