package sessions

import (
	"fmt"
	"testing"
	"time"
)

func TestPutPostDelete(t *testing.T) {
	s := NewStore()
	s.Put(Session{SessionID: "A", State: "OPEN", Peers: []Peer{{PeerID: 1}}})
	if s.Get("A") == nil {
		t.Fatal("missing")
	}
	if !s.AddPeer("A", Peer{PeerID: 2}) {
		t.Fatal("add")
	}
	if !s.DeletePeer("A", 2) {
		t.Fatal("del peer")
	}
	if s.Get("A").Peers[0].PeerID != 1 {
		t.Fatal("host remains")
	}
	s.Delete("A")
	if s.Get("A") != nil {
		t.Fatal("deleted")
	}
}

func TestMissingSession(t *testing.T) {
	s := NewStore()
	if s.AddPeer("nope", Peer{PeerID: 1}) {
		t.Fatal("expected fail")
	}
}

func TestMalformedUsername(t *testing.T) {
	s := NewStore()
	if _, ok := s.AuthKey("not-a-user", "blazium", "secret"); ok {
		t.Fatal("malformed")
	}
}

func TestExpiredSessionRejected(t *testing.T) {
	s := NewStore()
	exp := time.Now().Add(time.Hour).Unix()
	user := fmt.Sprintf("%d:ROOM:1", exp)
	s.Put(Session{SessionID: "ROOM", State: "OPEN", ExpiresAt: time.Now().Add(-time.Second).Unix(), Peers: []Peer{{PeerID: 1, Username: user}}})
	if _, ok := s.AuthKey(user, "blazium", "secret"); ok {
		t.Fatal("expired session must fail")
	}
	s.Put(Session{SessionID: "LIVE", State: "OPEN", ExpiresAt: 0, Peers: []Peer{{PeerID: 1, Username: user}}})
	// username session id is ROOM, not LIVE
	if _, ok := s.AuthKey(user, "blazium", "secret"); ok {
		t.Fatal("wrong session in username")
	}
	liveUser := fmt.Sprintf("%d:LIVE:1", exp)
	s.Put(Session{SessionID: "LIVE", State: "OPEN", ExpiresAt: 0, Peers: []Peer{{PeerID: 1, Username: liveUser}}})
	if _, ok := s.AuthKey(liveUser, "blazium", "secret"); !ok {
		t.Fatal("expires_at 0 must accept")
	}
}

func TestAuthKeyRejectsBadPeerKey(t *testing.T) {
	s := NewStore()
	exp := time.Now().Add(time.Hour).Unix()
	user := fmt.Sprintf("%d:ROOM:1", exp)
	s.Put(Session{SessionID: "ROOM", State: "OPEN", Peers: []Peer{{PeerID: 1, Username: user, Key: "@@@not-b64@@@"}}})
	if _, ok := s.AuthKey(user, "blazium", "secret"); ok {
		t.Fatal("malformed key must fail closed")
	}
}

func TestDeleteClearsSrc(t *testing.T) {
	s := NewStore()
	s.Put(Session{SessionID: "ROOM", State: "OPEN", Peers: []Peer{{PeerID: 1}}})
	s.RememberSrc("1:ROOM:1", "127.0.0.1:9")
	if s.UsernameForSrc("127.0.0.1:9") != "1:ROOM:1" {
		t.Fatal("remember")
	}
	s.Delete("ROOM")
	if s.UsernameForSrc("127.0.0.1:9") != "" {
		t.Fatal("src must be dropped with session")
	}
}
