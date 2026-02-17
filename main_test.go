package main

import (
	"os"
	"testing"
)

func TestParseEnv(t *testing.T) {
	raw := `
# comment
PORKBUN_API_KEY="pk_live"
PORKBUN_SECRET_KEY=sk_live
IGNORED
`
	vals := parseEnv(raw)
	if vals["PORKBUN_API_KEY"] != "pk_live" {
		t.Fatalf("expected api key, got %q", vals["PORKBUN_API_KEY"])
	}
	if vals["PORKBUN_SECRET_KEY"] != "sk_live" {
		t.Fatalf("expected secret key, got %q", vals["PORKBUN_SECRET_KEY"])
	}
}

func TestParseCheckResponse(t *testing.T) {
	data := map[string]any{
		"status": "SUCCESS",
		"response": map[string]any{
			"avail": "yes",
			"pricing": map[string]any{
				"registration": "12.34",
				"renewal":      "14.56",
			},
		},
	}
	avail, price, renewal := parseCheckResponse(data)
	if !avail {
		t.Fatal("expected available=true")
	}
	if price != "12.34" {
		t.Fatalf("expected price=12.34, got %q", price)
	}
	if renewal != "14.56" {
		t.Fatalf("expected renewal=14.56, got %q", renewal)
	}
}

func TestLoadKeysPrefersEnvironment(t *testing.T) {
	prevAPI := os.Getenv("PORKBUN_API_KEY")
	prevSecret := os.Getenv("PORKBUN_SECRET_KEY")
	prevPath := os.Getenv("PORKCTL_ENV_FILE")
	t.Cleanup(func() {
		if prevAPI == "" {
			_ = os.Unsetenv("PORKBUN_API_KEY")
		} else {
			_ = os.Setenv("PORKBUN_API_KEY", prevAPI)
		}
		if prevSecret == "" {
			_ = os.Unsetenv("PORKBUN_SECRET_KEY")
		} else {
			_ = os.Setenv("PORKBUN_SECRET_KEY", prevSecret)
		}
		if prevPath == "" {
			_ = os.Unsetenv("PORKCTL_ENV_FILE")
		} else {
			_ = os.Setenv("PORKCTL_ENV_FILE", prevPath)
		}
	})

	_ = os.Setenv("PORKBUN_API_KEY", "env_pk")
	_ = os.Setenv("PORKBUN_SECRET_KEY", "env_sk")
	_ = os.Setenv("PORKCTL_ENV_FILE", "C:/this/path/does/not/exist.env")

	keys, err := loadKeys()
	if err != nil {
		t.Fatalf("loadKeys returned error: %v", err)
	}
	if keys.APIKey != "env_pk" || keys.SecretKey != "env_sk" {
		t.Fatalf("unexpected keys: %#v", keys)
	}
}
