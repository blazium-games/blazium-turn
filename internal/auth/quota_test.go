package auth

import (
	"testing"

	"github.com/blazium-games/blazium-turn/internal/sessions"
)

func TestQuotaOneAllocation(t *testing.T) {
	store := sessions.NewStore()
	store.Put(sessions.Session{SessionID: "R", Peers: []sessions.Peer{{PeerID: 1, Username: "u"}}})
	q := Quota(store)
	if !q("u") {
		t.Fatal("first")
	}
	store.MarkAllocated("u", "1.2.3.4")
	if q("u") {
		t.Fatal("second must fail")
	}
}
