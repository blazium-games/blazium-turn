# blazium-turn

Open-source Go reference for session-aware TURN next to [blazium-signal](https://github.com/blazium-games/blazium-signal). Not stock coturn. Not a hosted product.

Signaling passes live sessions in on create/join and revokes them on leave/kick/close. Auth, permission, and quota hooks refuse allocations that do not belong to a live session.

- Engine module: `games_enet_webrtc` in [blazium](https://github.com/blazium-games/blazium)
- Companion signaling: [blazium-signal](https://github.com/blazium-games/blazium-signal)
- Module tests: [games_enet_webrtc_module_tests](https://github.com/blazium-games/games_enet_webrtc_module_tests)

## Run

```text
go run ./cmd/turn
```

Defaults (open / local): TURN `127.0.0.1:3478`, control `127.0.0.1:3479`, `-auth=open` (no token on the control API), no TLS.

```text
go test ./...
```

When `-internal-token` is empty, this service uses a shared development default that matches [blazium-signal](https://github.com/blazium-games/blazium-signal) in open mode. Set a matching token on any non-local deploy.

TLS / TURNS is not served by this binary. Put a TLS front in front of the control API if you expose it beyond loopback.

## Control API (signaling only)

| Method | Path |
|--------|------|
| PUT | `/internal/sessions/{id}` |
| POST | `/internal/sessions/{id}/peers` |
| DELETE | `/internal/sessions/{id}/peers/{peer_id}` |
| DELETE | `/internal/sessions/{id}` |
| GET | `/internal/sessions/{id}/stats` |

Username: `{unix_expiry}:{session_id}:{peer_id}`. After DELETE peer, Refresh fails even if clock TTL remains.

## Docker

```text
docker build -t blazium-turn .
docker run --rm -p 3478:3478/udp -p 3478:3478/tcp -p 3479:3479 blazium-turn
```

To run signal and TURN together, clone this repository as a sibling of [blazium-signal](https://github.com/blazium-games/blazium-signal) and use that repo's `deploy/docker-compose.yml`.

## License

MIT — see [LICENSE](LICENSE).
