package oplog

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewIDSortableAndUnique(t *testing.T) {
	a := NewID()
	time.Sleep(2 * time.Millisecond)
	b := NewID()
	if len(a) != 26 || len(b) != 26 {
		t.Fatalf("lengths %d %d", len(a), len(b))
	}
	if !(a < b) {
		t.Errorf("IDs not time-ordered: %s !< %s", a, b)
	}
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := NewID()
		if seen[id] {
			t.Fatal("duplicate ID")
		}
		seen[id] = true
	}
}

func TestRedact(t *testing.T) {
	got := Redact([]string{"--token=abc123", "API_KEY:xyz", "https://user:pw@host/x", "plain"})
	joined := strings.Join(got, " ")
	for _, leak := range []string{"abc123", "xyz", "user:pw"} {
		if strings.Contains(joined, leak) {
			t.Errorf("secret %q leaked: %s", leak, joined)
		}
	}
	if got[3] != "plain" {
		t.Error("ordinary args must be unchanged")
	}
}

func TestS7_IntentWithoutOutcomeIsIncomplete(t *testing.T) {
	t.Setenv("THINE_HOME", t.TempDir())
	t.Setenv("THINE_STATE_DIR", "")
	id := NewID()
	if err := Append(Record{OpID: id, Phase: "intent", Verb: "cleanup", Caller: "human",
		Items: []Item{{ID: "a", Action: "quarantine", Target: "/x"}}}); err != nil {
		t.Fatal(err)
	}
	ops, err := Ops()
	if err != nil || len(ops) != 1 || ops[0].Result != "incomplete" || len(ops[0].Items) != 1 {
		t.Fatalf("crash evidence missing: %+v %v", ops, err)
	}
	Append(Record{OpID: id, Phase: "outcome", Verb: "cleanup", Result: "completed",
		Items: []Item{{ID: "a", Result: "done", Undoable: true}}})
	op, err := Find(id[:12])
	if err != nil || op.Result != "completed" || !op.Undoable {
		t.Fatalf("outcome not merged: %+v %v", op, err)
	}
	st, _ := os.Stat(LogPath())
	if st.Mode().Perm() != 0o600 {
		t.Errorf("ops log mode %v, want 0600", st.Mode().Perm())
	}
}
