package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseEnv(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Setenv("PORKBUN_API_KEY", "env_pk")
	t.Setenv("PORKBUN_SECRET_KEY", "env_sk")
	t.Setenv("PORKCTL_ENV_FILE", "C:/this/path/does/not/exist.env")

	keys, err := loadKeys()
	if err != nil {
		t.Fatalf("loadKeys returned error: %v", err)
	}
	if keys.APIKey != "env_pk" || keys.SecretKey != "env_sk" {
		t.Fatalf("unexpected keys: %#v", keys)
	}
}

func TestResolveKeysFromConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.json")
	cfg := map[string]string{"api_key": "cfg_pk", "secret_key": "cfg_sk"}
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgFile, b, 0o600); err != nil {
		t.Fatal(err)
	}

	keys, err := loadKeysFromConfig(cfgFile)
	if err != nil {
		t.Fatalf("loadKeysFromConfig error: %v", err)
	}
	if keys.APIKey != "cfg_pk" || keys.SecretKey != "cfg_sk" {
		t.Fatalf("unexpected keys: %#v", keys)
	}
}

func TestLoadKeysFromFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	envFile := filepath.Join(dir, "porkbun.env")
	if err := os.WriteFile(envFile, []byte("PORKBUN_API_KEY=file_pk\nPORKBUN_SECRET_KEY=file_sk\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	keys, err := loadKeysFromFile(envFile)
	if err != nil {
		t.Fatalf("loadKeysFromFile error: %v", err)
	}
	if keys.APIKey != "file_pk" || keys.SecretKey != "file_sk" {
		t.Fatalf("unexpected keys: %#v", keys)
	}
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()
	// Just verify it doesn't error on a simple value
	// (stdout capture not needed — we just test no panic/error)
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := writeJSON(map[string]any{"foo": "bar"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("writeJSON error: %v", err)
	}
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	got := string(buf[:n])
	if got == "" {
		t.Fatal("expected output from writeJSON")
	}
}

func TestMaskKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"pk1_abcdefghij", "pk1_...ghij"},
		{"short", "*****"},
		{"12345678", "********"},
		{"123456789", "1234...6789"},
	}
	for _, tt := range tests {
		got := maskKey(tt.input)
		if got != tt.want {
			t.Errorf("maskKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRunVersionText(t *testing.T) {
	t.Parallel()
	err := run([]string{"version"})
	if err != nil {
		t.Fatalf("run version: %v", err)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	t.Parallel()
	err := run([]string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestRunNoArgs(t *testing.T) {
	t.Parallel()
	err := run(nil)
	if err != nil {
		t.Fatalf("run with no args should print usage, got error: %v", err)
	}
}

func TestConfigPath(t *testing.T) {
	t.Parallel()
	p, err := configPath()
	if err != nil {
		t.Fatalf("configPath error: %v", err)
	}
	if !filepath.IsAbs(p) {
		t.Fatalf("expected absolute path, got %q", p)
	}
	if filepath.Base(p) != "porkctl" {
		t.Fatalf("expected path ending in 'porkctl', got %q", p)
	}
}
