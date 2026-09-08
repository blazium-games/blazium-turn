package control

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blazium-games/blazium-turn/internal/config"
	"github.com/blazium-games/blazium-turn/internal/sessions"
)

func TestHealthz(t *testing.T) {
	srv := httptest.NewServer(Handler(config.Config{Auth: "open"}, sessions.NewStore()))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body bytes.Buffer
	if _, err := body.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body.Bytes(), []byte(`{"ok":true}`)) {
		t.Fatalf("healthz %s", body.String())
	}
}

func TestOpenAndLocked(t *testing.T) {
	store := sessions.NewStore()
	open := httptest.NewServer(Handler(config.Config{Auth: "open"}, store))
	defer open.Close()
	body, _ := json.Marshal(sessions.Session{SessionID: "S1", State: "OPEN"})
	resp, err := http.Post(open.URL+"/internal/sessions/S1", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	req, _ := http.NewRequest(http.MethodPut, open.URL+"/internal/sessions/S1", bytes.NewReader(body))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 204 {
		t.Fatalf("open put %d", resp.StatusCode)
	}
	resp.Body.Close()

	locked := httptest.NewServer(Handler(config.Config{Auth: "token", InternalToken: "tok"}, sessions.NewStore()))
	defer locked.Close()
	req, _ = http.NewRequest(http.MethodPut, locked.URL+"/internal/sessions/S1", bytes.NewReader(body))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", resp.StatusCode)
	}
	req, _ = http.NewRequest(http.MethodPut, locked.URL+"/internal/sessions/S1", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("locked put %d", resp.StatusCode)
	}
}

func TestRejectsBadIDBodyAndPeer(t *testing.T) {
	store := sessions.NewStore()
	open := httptest.NewServer(Handler(config.Config{Auth: "open"}, store))
	defer open.Close()
	body, _ := json.Marshal(sessions.Session{SessionID: "S1", State: "OPEN"})
	req, _ := http.NewRequest(http.MethodPut, open.URL+"/internal/sessions/..%2Fx", bytes.NewReader(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("dotdot %d", resp.StatusCode)
	}
	req, _ = http.NewRequest(http.MethodPut, open.URL+"/internal/sessions/S1", bytes.NewReader(bytes.Repeat([]byte("a"), 40<<10)))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("oversize %d", resp.StatusCode)
	}
	ok, _ := json.Marshal(sessions.Session{SessionID: "S1", State: "OPEN", Peers: []sessions.Peer{{PeerID: 1}}})
	req, _ = http.NewRequest(http.MethodPut, open.URL+"/internal/sessions/S1", bytes.NewReader(ok))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 204 {
		t.Fatalf("valid put %d", resp.StatusCode)
	}
	resp.Body.Close()
	badPeer, _ := json.Marshal(sessions.Peer{PeerID: 0})
	resp, err = http.Post(open.URL+"/internal/sessions/S1/peers", "application/json", bytes.NewReader(badPeer))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("peer 0 %d", resp.StatusCode)
	}
	badPeer, _ = json.Marshal(sessions.Peer{PeerID: 255})
	resp, err = http.Post(open.URL+"/internal/sessions/S1/peers", "application/json", bytes.NewReader(badPeer))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("peer 255 %d", resp.StatusCode)
	}
}

func TestRejectsPastExpiryAndBadUsername(t *testing.T) {
	store := sessions.NewStore()
	open := httptest.NewServer(Handler(config.Config{Auth: "open"}, store))
	defer open.Close()
	past, _ := json.Marshal(sessions.Session{SessionID: "S1", State: "OPEN", ExpiresAt: 1})
	req, _ := http.NewRequest(http.MethodPut, open.URL+"/internal/sessions/S1", bytes.NewReader(past))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("past expires_at %d", resp.StatusCode)
	}
	badUser, _ := json.Marshal(sessions.Session{
		SessionID: "S2", State: "OPEN",
		Peers: []sessions.Peer{{PeerID: 1, Username: "not-a-user"}},
	})
	req, _ = http.NewRequest(http.MethodPut, open.URL+"/internal/sessions/S2", bytes.NewReader(badUser))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("bad username %d", resp.StatusCode)
	}
	okUser, _ := json.Marshal(sessions.Session{
		SessionID: "S3", State: "OPEN",
		Peers: []sessions.Peer{{PeerID: 1, Username: "9999999999:S3:1"}},
	})
	req, _ = http.NewRequest(http.MethodPut, open.URL+"/internal/sessions/S3", bytes.NewReader(okUser))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("valid username %d", resp.StatusCode)
	}
}

func TestRejectsBadFakeIPAndState(t *testing.T) {
	store := sessions.NewStore()
	open := httptest.NewServer(Handler(config.Config{Auth: "open"}, store))
	defer open.Close()
	badIP, _ := json.Marshal(sessions.Session{
		SessionID: "S4", State: "OPEN",
		Peers: []sessions.Peer{{PeerID: 1, FakeIP: "8.8.8.8"}},
	})
	req, _ := http.NewRequest(http.MethodPut, open.URL+"/internal/sessions/S4", bytes.NewReader(badIP))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("bad fake_ip %d", resp.StatusCode)
	}
	badState, _ := json.Marshal(sessions.Session{SessionID: "S5", State: "bogus"})
	req, _ = http.NewRequest(http.MethodPut, open.URL+"/internal/sessions/S5", bytes.NewReader(badState))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("bad state %d", resp.StatusCode)
	}
	okIP, _ := json.Marshal(sessions.Session{
		SessionID: "S6", State: "OPEN",
		Peers: []sessions.Peer{{PeerID: 1, FakeIP: "10.66.0.1"}},
	})
	req, _ = http.NewRequest(http.MethodPut, open.URL+"/internal/sessions/S6", bytes.NewReader(okIP))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("valid fake_ip %d", resp.StatusCode)
	}
}
