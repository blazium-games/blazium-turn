package sessions

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Peer struct {
	PeerID     int    `json:"peer_id"`
	FakeIP     string `json:"fake_ip"`
	FakePort   int    `json:"fake_port"`
	Username   string `json:"turn_username"`
	Key        string `json:"turn_key"`
	RelayAddr  string `json:"relay_addr,omitempty"`
	Allocated  bool   `json:"allocated,omitempty"`
}

type Session struct {
	SessionID string `json:"session_id"`
	GameID    string `json:"game_id"`
	RoomCode  string `json:"room_code"`
	State     string `json:"state"`
	ExpiresAt int64  `json:"expires_at"`
	MaxPeers  int    `json:"max_peers"`
	Peers     []Peer `json:"peers"`
}

type Store struct {
	mu   sync.Mutex
	data map[string]*Session
	src  map[string]string
}

func NewStore() *Store { return &Store{data: map[string]*Session{}, src: map[string]string{}} }

func (s *Store) RememberSrc(username, src string) {
	if username == "" || src == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.src[src] = username
}

func (s *Store) UsernameForSrc(src string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.src[src]
}

func (s *Store) Put(sess Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess.State == "" {
		sess.State = "OPEN"
	}
	cur := s.data[sess.SessionID]
	if cur != nil && sess.State == "SEALED" {
		cur.State = "SEALED"
		return
	}
	cp := sess
	s.data[sess.SessionID] = &cp
}

func (s *Store) Get(id string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[id]
}

func (s *Store) forgetSrcLocked(match func(username string) bool) {
	for src, user := range s.src {
		if match(user) {
			delete(s.src, src)
		}
	}
}

func (s *Store) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forgetSrcLocked(func(user string) bool {
		_, sid, _, ok := ParseUsername(user)
		return ok && sid == id
	})
	delete(s.data, id)
}

func (s *Store) AddPeer(id string, p Peer) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.data[id]
	if sess == nil || sess.State == "CLOSING" {
		return false
	}
	for i := range sess.Peers {
		if sess.Peers[i].PeerID == p.PeerID {
			sess.Peers[i] = p
			return true
		}
	}
	sess.Peers = append(sess.Peers, p)
	return true
}

func (s *Store) DeletePeer(id string, peerID int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.data[id]
	if sess == nil {
		return false
	}
	out := sess.Peers[:0]
	for _, p := range sess.Peers {
		if p.PeerID != peerID {
			out = append(out, p)
		}
	}
	sess.Peers = out
	s.forgetSrcLocked(func(user string) bool {
		_, sid, pid, ok := ParseUsername(user)
		return ok && sid == id && pid == peerID
	})
	return true
}

func (s *Store) MarkAllocated(username, relay string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sess := range s.data {
		for i := range sess.Peers {
			if sess.Peers[i].Username == username {
				sess.Peers[i].Allocated = true
				sess.Peers[i].RelayAddr = relay
			}
		}
	}
}

func (s *Store) HasAllocation(username string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sess := range s.data {
		for _, p := range sess.Peers {
			if p.Username == username && p.Allocated {
				return true
			}
		}
	}
	return false
}

func ParseUsername(username string) (expiry int64, sessionID string, peerID int, ok bool) {
	parts := strings.Split(username, ":")
	if len(parts) != 3 {
		return 0, "", 0, false
	}
	exp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", 0, false
	}
	pid, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, "", 0, false
	}
	return exp, parts[1], pid, true
}

func (s *Store) AuthKey(username, realm, hmacSecret string) ([]byte, bool) {
	exp, sessionID, peerID, ok := ParseUsername(username)
	if !ok {
		return nil, false
	}
	if time.Now().Unix() > exp {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.data[sessionID]
	if sess == nil || sess.State == "CLOSING" {
		return nil, false
	}
	if sess.ExpiresAt > 0 && time.Now().Unix() > sess.ExpiresAt {
		return nil, false
	}
	var peer *Peer
	for i := range sess.Peers {
		if sess.Peers[i].PeerID == peerID {
			peer = &sess.Peers[i]
			break
		}
	}
	if peer == nil {
		return nil, false
	}
	if peer.Key != "" {
		raw, err := base64.StdEncoding.DecodeString(peer.Key)
		if err != nil {
			return nil, false
		}
		return raw, true
	}
	mac := hmac.New(sha1.New, []byte(hmacSecret))
	mac.Write([]byte(username))
	return mac.Sum(nil), true
}

func (s *Store) PermitDest(username, destIP string, strict bool) bool {
	if !strict {
		_, sessionID, _, ok := ParseUsername(username)
		if !ok {
			return false
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.data[sessionID] != nil
	}
	_, sessionID, peerID, ok := ParseUsername(username)
	if !ok {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.data[sessionID]
	if sess == nil {
		return false
	}
	for _, p := range sess.Peers {
		if p.PeerID == peerID {
			continue
		}
		if p.RelayAddr == destIP || strings.HasPrefix(p.RelayAddr, destIP+":") {
			return true
		}
	}
	return false
}
