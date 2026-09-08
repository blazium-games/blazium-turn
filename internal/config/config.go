package config

import (
	"flag"
	"os"
	"strconv"
)

type Config struct {
	Addr          string
	ControlAddr   string
	Realm         string
	Auth          string // open | token
	InternalToken string
	TLS           bool
	PublicIP      string
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func Parse() Config {
	c := Config{
		Addr:          envOr("TURN_ADDR", "127.0.0.1:3478"),
		ControlAddr:   envOr("TURN_CONTROL_ADDR", "127.0.0.1:3479"),
		Realm:         envOr("TURN_REALM", "blazium"),
		Auth:          envOr("TURN_AUTH", "open"),
		InternalToken: envOr("TURN_INTERNAL_TOKEN", ""),
		TLS:           envBool("TURN_TLS", false),
		PublicIP:      envOr("TURN_PUBLIC_IP", "127.0.0.1"),
	}
	flag.StringVar(&c.Addr, "addr", c.Addr, "TURN listen address")
	flag.StringVar(&c.ControlAddr, "control-addr", c.ControlAddr, "internal HTTP")
	flag.StringVar(&c.Realm, "realm", c.Realm, "TURN realm")
	flag.StringVar(&c.Auth, "auth", c.Auth, "open | token")
	flag.StringVar(&c.InternalToken, "internal-token", c.InternalToken, "control API token")
	flag.BoolVar(&c.TLS, "tls", c.TLS, "enable TURNS")
	flag.StringVar(&c.PublicIP, "public-ip", c.PublicIP, "relay advertised IP")
	flag.Parse()
	return c
}
