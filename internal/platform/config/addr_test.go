package config_test

import (
	"testing"

	"prestamos/internal/platform/config"
)

func addrEnv(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func addrBase() map[string]string {
	return map[string]string{
		"DATABASE_URL": "postgres://u:p@localhost:5432/db?sslmode=disable",
		"JWT_SECRET":   "0123456789abcdef0123456789abcdef",
	}
}

func TestLoad_AddrFromServerHostAndPort(t *testing.T) {
	m := addrBase()
	m["SERVER_HOST"], m["SERVER_PORT"] = "127.0.0.1", "9090"
	c, err := config.Load(addrEnv(m))
	if err != nil || c.HTTPAddr != "127.0.0.1:9090" {
		t.Fatalf("addr=%q err=%v", c.HTTPAddr, err)
	}
}

func TestLoad_HTTPAddrWinsOverServerVars(t *testing.T) {
	m := addrBase()
	m["SERVER_HOST"], m["SERVER_PORT"], m["HTTP_ADDR"] = "127.0.0.1", "9090", ":7000"
	c, err := config.Load(addrEnv(m))
	if err != nil || c.HTTPAddr != ":7000" {
		t.Fatalf("addr=%q err=%v", c.HTTPAddr, err)
	}
}

func TestLoad_DefaultAddrWhenNothingSet(t *testing.T) {
	c, err := config.Load(addrEnv(addrBase()))
	if err != nil || c.HTTPAddr != ":8080" {
		t.Fatalf("addr=%q err=%v", c.HTTPAddr, err)
	}
}
