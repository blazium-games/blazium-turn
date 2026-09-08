package auth

import (
	"net"

	"github.com/blazium-games/blazium-turn/internal/sessions"
	"github.com/pion/turn/v4"
)

func Handler(store *sessions.Store, realm, secret string) turn.AuthHandler {
	return func(username, realmName string, srcAddr net.Addr) ([]byte, bool) {
		_ = realmName
		key, ok := store.AuthKey(username, realm, secret)
		if ok && srcAddr != nil {
			store.RememberSrc(username, srcAddr.String())
		}
		return key, ok
	}
}

func DeniedPeerIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
		return true
	}
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return true
	}
	if v4[0] == 100 && v4[1] == 100 && v4[2] == 100 && v4[3] == 200 {
		return true
	}
	return false
}

func PermissionHandler(store *sessions.Store) turn.PermissionHandler {
	return func(clientAddr net.Addr, peerIP net.IP) bool {
		if clientAddr == nil || DeniedPeerIP(peerIP) {
			return false
		}
		user := store.UsernameForSrc(clientAddr.String())
		if user == "" {
			return false
		}
		return store.PermitDest(user, peerIP.String(), false)
	}
}

func Permission(store *sessions.Store, strict bool) func(username, destIP string) bool {
	return func(username, destIP string) bool {
		return store.PermitDest(username, destIP, strict)
	}
}

func Quota(store *sessions.Store) func(username string) bool {
	return func(username string) bool {
		return !store.HasAllocation(username)
	}
}

func QuotaHandler(store *sessions.Store) turn.QuotaHandler {
	inner := Quota(store)
	return func(username, realm string, srcAddr net.Addr) bool {
		_ = realm
		_ = srcAddr
		return inner(username)
	}
}

func EventHandler(store *sessions.Store) turn.EventHandler {
	return turn.EventHandler{
		OnAllocationCreated: func(src, dst net.Addr, proto, username, realm string, relayAddr net.Addr, requestedPort int) {
			_ = src
			_ = dst
			_ = proto
			_ = realm
			_ = requestedPort
			relay := ""
			if relayAddr != nil {
				relay = relayAddr.String()
			}
			store.MarkAllocated(username, relay)
		},
	}
}
