package control

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/blazium-games/blazium-turn/internal/config"
	"github.com/blazium-games/blazium-turn/internal/sessions"
)

func Handler(cfg config.Config, store *sessions.Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/internal/sessions/", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Auth != "open" {
			tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if tok == "" || tok != cfg.InternalToken {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
		path := strings.TrimPrefix(r.URL.Path, "/internal/sessions/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		id := parts[0]
		if err := validID(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		switch {
		case r.Method == http.MethodPut && len(parts) == 1:
			var sess sessions.Session
			if err := json.NewDecoder(r.Body).Decode(&sess); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			if sess.SessionID == "" {
				sess.SessionID = id
			}
			if err := validID(sess.SessionID); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			if sess.MaxPeers < 0 || sess.MaxPeers > 254 {
				http.Error(w, "max_peers", 400)
				return
			}
			if sess.ExpiresAt != 0 && sess.ExpiresAt < time.Now().Unix() {
				http.Error(w, "expires_at", 400)
				return
			}
			if err := validState(sess.State); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			if err := validGameID(sess.GameID); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			for _, p := range sess.Peers {
				if p.PeerID < 1 || p.PeerID > 254 {
					http.Error(w, "peer_id", 400)
					return
				}
				if err := validTurnUsername(p.Username, p.PeerID); err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
				if err := validFakeIP(p.FakeIP, p.PeerID); err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
			}
			store.Put(sess)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "peers":
			var p sessions.Peer
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			if p.PeerID < 1 || p.PeerID > 254 {
				http.Error(w, "peer_id", 400)
				return
			}
			if err := validTurnUsername(p.Username, p.PeerID); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			if err := validFakeIP(p.FakeIP, p.PeerID); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			if !store.AddPeer(id, p) {
				http.Error(w, "missing session", http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && len(parts) == 3 && parts[1] == "peers":
			pid := atoi(parts[2])
			if pid < 1 || pid > 254 {
				http.Error(w, "peer_id", 400)
				return
			}
			store.DeletePeer(id, pid)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && len(parts) == 1:
			store.Delete(id)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && len(parts) == 2 && parts[1] == "stats":
			sess := store.Get(id)
			if sess == nil {
				http.Error(w, "missing", 404)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(sess)
		default:
			http.NotFound(w, r)
		}
	})
	return mux
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func validState(s string) error {
	switch s {
	case "", "OPEN", "SEALED", "CLOSING":
		return nil
	default:
		return errBadState
	}
}

func validGameID(s string) error {
	if s == "" {
		return nil
	}
	if len(s) > 64 || strings.ContainsAny(s, "\r\n") {
		return errBadGame
	}
	for _, r := range s {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-') {
			return errBadGame
		}
	}
	return nil
}

func validFakeIP(ip string, peerID int) error {
	if ip == "" {
		return nil
	}
	want := "10.66.0." + itoa(peerID)
	if ip != want {
		return errBadIP
	}
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func validTurnUsername(username string, peerID int) error {
	if username == "" {
		return nil
	}
	_, _, pid, ok := sessions.ParseUsername(username)
	if !ok || pid != peerID {
		return errBadUser
	}
	return nil
}

func validID(id string) error {
	if id == "" || len(id) > 64 || strings.ContainsAny(id, "/.\\") {
		return errBadID
	}
	for _, r := range id {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return errBadID
		}
	}
	return nil
}

type idError string

func (e idError) Error() string { return string(e) }

const errBadID idError = "invalid session_id"
const errBadUser idError = "invalid turn_username"
const errBadState idError = "invalid state"
const errBadGame idError = "invalid game_id"
const errBadIP idError = "invalid fake_ip"
