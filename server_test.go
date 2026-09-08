package turn_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/blazium-games/blazium-turn/internal/sessions"
	"github.com/pion/turn/v4"
)

func TestAllocateMintedAndStale(t *testing.T) {
	store := sessions.NewStore()
	exp := time.Now().Add(time.Hour).Unix()
	user := fmt.Sprintf("%d:ROOM:1", exp)
	store.Put(sessions.Session{SessionID: "ROOM", State: "OPEN", Peers: []sessions.Peer{{PeerID: 1, Username: user}}})

	udp, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	s, err := turn.NewServer(turn.ServerConfig{
		Realm: "blazium",
		AuthHandler: func(username, realm string, srcAddr net.Addr) ([]byte, bool) {
			return store.AuthKey(username, realm, "dev-open-secret")
		},
		QuotaHandler: func(username, realm string, srcAddr net.Addr) bool {
			return !store.HasAllocation(username)
		},
		EventHandler: turn.EventHandler{
			OnAllocationCreated: func(src, dst net.Addr, proto, username, realm string, relayAddr net.Addr, requestedPort int) {
				relay := ""
				if relayAddr != nil {
					relay = relayAddr.String()
				}
				store.MarkAllocated(username, relay)
			},
		},
		PacketConnConfigs: []turn.PacketConnConfig{{
			PacketConn: udp,
			RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
				RelayAddress: net.ParseIP("127.0.0.1"),
				Address:      "127.0.0.1",
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	key, ok := store.AuthKey(user, "blazium", "dev-open-secret")
	if !ok || len(key) == 0 {
		t.Fatal("minted cred must auth")
	}
	stale := fmt.Sprintf("%d:ROOM:9", exp)
	if _, ok := store.AuthKey(stale, "blazium", "dev-open-secret"); ok {
		t.Fatal("stale peer must fail")
	}
}
