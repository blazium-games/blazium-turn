package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/blazium-games/blazium-turn/internal/auth"
	"github.com/blazium-games/blazium-turn/internal/config"
	"github.com/blazium-games/blazium-turn/internal/control"
	"github.com/blazium-games/blazium-turn/internal/sessions"
	"github.com/pion/turn/v4"
)

func main() {
	cfg := config.Parse()
	store := sessions.NewStore()
	secret := cfg.InternalToken
	if secret == "" {
		secret = "dev-open-secret"
	}

	httpSrv := &http.Server{
		Addr:              cfg.ControlAddr,
		Handler:           control.Handler(cfg, store),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	go func() {
		log.Printf("blazium-turn control %s auth=%s", cfg.ControlAddr, cfg.Auth)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	udpAddr, err := net.ResolveUDPAddr("udp", cfg.Addr)
	if err != nil {
		log.Fatal(err)
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal(err)
	}
	tcpAddr, err := net.ResolveTCPAddr("tcp", cfg.Addr)
	if err != nil {
		log.Fatal(err)
	}
	tcpLn, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		log.Fatal(err)
	}

	s, err := turn.NewServer(turn.ServerConfig{
		Realm:        cfg.Realm,
		AuthHandler:  auth.Handler(store, cfg.Realm, secret),
		QuotaHandler: auth.QuotaHandler(store),
		EventHandler: auth.EventHandler(store),
		PacketConnConfigs: []turn.PacketConnConfig{{
			PacketConn: conn,
			PermissionHandler: auth.PermissionHandler(store),
			RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
				RelayAddress: net.ParseIP(cfg.PublicIP),
				Address:      "0.0.0.0",
			},
		}},
		ListenerConfigs: []turn.ListenerConfig{{
			Listener: tcpLn,
			PermissionHandler: auth.PermissionHandler(store),
			RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
				RelayAddress: net.ParseIP(cfg.PublicIP),
				Address:      "0.0.0.0",
			},
		}},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("blazium-turn example TURN on %s realm=%s", cfg.Addr, cfg.Realm)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
	if err := s.Close(); err != nil {
		log.Printf("turn close: %v", err)
	}
}
