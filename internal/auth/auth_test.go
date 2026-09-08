package auth

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/blazium-games/blazium-turn/internal/sessions"
)

func TestAuthAcceptReject(t *testing.T) {
	store := sessions.NewStore()
	exp := time.Now().Add(time.Hour).Unix()
	user := fmt.Sprintf("%d:ROOM:2", exp)
	mac := hmac.New(sha1.New, []byte("secret"))
	mac.Write([]byte(user))
	key := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	store.Put(sessions.Session{
		SessionID: "ROOM", State: "OPEN",
		Peers: []sessions.Peer{{PeerID: 2, Username: user, Key: key}},
	})
	h := Handler(store, "blazium", "secret")
	if _, ok := h(user, "blazium", nil); !ok {
		t.Fatal("expected accept")
	}
	other := fmt.Sprintf("%d:OTHER:2", exp)
	if _, ok := h(other, "blazium", nil); ok {
		t.Fatal("other room must fail")
	}
	store.DeletePeer("ROOM", 2)
	if _, ok := h(user, "blazium", nil); ok {
		t.Fatal("deleted peer must fail while TTL valid")
	}
}

func TestQuotaAndPermission(t *testing.T) {
	store := sessions.NewStore()
	store.Put(sessions.Session{SessionID: "R", State: "OPEN", Peers: []sessions.Peer{
		{PeerID: 1, Username: "1:R:1", RelayAddr: "10.0.0.1"},
		{PeerID: 2, Username: "1:R:2"},
	}})
	q := Quota(store)
	if !q("1:R:2") {
		t.Fatal("first alloc ok")
	}
	store.MarkAllocated("1:R:2", "10.0.0.2")
	if q("1:R:2") {
		t.Fatal("second alloc rejected")
	}
	perm := Permission(store, true)
	if !perm("1:R:2", "10.0.0.1") {
		t.Fatal("same session dest allowed")
	}
	if perm("1:R:2", "8.8.8.8") {
		t.Fatal("foreign dest denied")
	}
}

func TestPermissionHandlerSameSession(t *testing.T) {
	store := sessions.NewStore()
	store.Put(sessions.Session{SessionID: "R", State: "OPEN", Peers: []sessions.Peer{
		{PeerID: 1, Username: "1:R:1"},
		{PeerID: 2, Username: "1:R:2"},
	}})
	src := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9}
	store.RememberSrc("1:R:2", src.String())
	h := PermissionHandler(store)
	if !h(src, net.IPv4(8, 8, 8, 8)) {
		t.Fatal("strict=false allows any dest while session is live")
	}
	if h(&net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 9}, net.IPv4(8, 8, 8, 8)) {
		t.Fatal("unknown client src denied")
	}
	store.Delete("R")
	if h(src, net.IPv4(8, 8, 8, 8)) {
		t.Fatal("deleted session denied")
	}
}

func TestPermissionHandlerDeniesLoopbackAndLinkLocal(t *testing.T) {
	store := sessions.NewStore()
	store.Put(sessions.Session{SessionID: "R", State: "OPEN", Peers: []sessions.Peer{{PeerID: 1, Username: "1:R:1"}}})
	src := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9}
	store.RememberSrc("1:R:1", src.String())
	h := PermissionHandler(store)
	if h(src, net.IPv4(127, 0, 0, 1)) {
		t.Fatal("loopback dest")
	}
	if h(src, net.ParseIP("169.254.169.254")) {
		t.Fatal("link-local dest")
	}
	if !h(src, net.IPv4(8, 8, 8, 8)) {
		t.Fatal("public dest should still be allowed")
	}
	if h(src, net.IPv4(10, 0, 0, 1)) {
		t.Fatal("rfc1918 dest")
	}
	if h(src, net.IPv4(100, 64, 0, 1)) {
		t.Fatal("cgnat dest")
	}
	if h(src, net.IPv4(0, 0, 0, 0)) {
		t.Fatal("unspecified dest")
	}
	if h(src, net.IPv4(224, 0, 0, 1)) {
		t.Fatal("multicast dest")
	}
	if h(src, net.IPv4(100, 100, 100, 200)) {
		t.Fatal("100.100.100.200 dest")
	}
	if !DeniedPeerIP(net.ParseIP("::")) {
		t.Fatal("ipv6 unspecified")
	}
	if !DeniedPeerIP(net.ParseIP("ff02::1")) {
		t.Fatal("ipv6 multicast")
	}
}

func TestQuotaHandlerAndEventHandler(t *testing.T) {
	store := sessions.NewStore()
	store.Put(sessions.Session{SessionID: "R", State: "OPEN", Peers: []sessions.Peer{
		{PeerID: 1, Username: "1:R:1"},
	}})
	qh := QuotaHandler(store)
	if !qh("1:R:1", "blazium", nil) {
		t.Fatal("first quota ok")
	}
	store.MarkAllocated("1:R:1", "8.8.8.8:1")
	if qh("1:R:1", "blazium", nil) {
		t.Fatal("second quota denied")
	}

	store2 := sessions.NewStore()
	store2.Put(sessions.Session{SessionID: "S", State: "OPEN", Peers: []sessions.Peer{
		{PeerID: 2, Username: "1:S:2"},
	}})
	eh := EventHandler(store2)
	relay := &net.UDPAddr{IP: net.IPv4(8, 8, 8, 8), Port: 3478}
	eh.OnAllocationCreated(nil, nil, "udp", "1:S:2", "blazium", relay, 3478)
	if !store2.HasAllocation("1:S:2") {
		t.Fatal("event handler should mark allocated")
	}
}
