package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	t.Setenv("T_PORKBUN_ENV_FILE", "C:/this/path/does/not/exist.env")

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
	if filepath.Base(p) != "t-porkbun" {
		t.Fatalf("expected path ending in 't-porkbun', got %q", p)
	}
}

// --- Fuzzy matching tests ---

func TestFuzzyMatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text    string
		pattern string
		want    int
	}{
		{"A www.example.com", "a www.example.com", 0}, // exact (case-insensitive)
		{"A www.example.com", "A www", 1},              // prefix
		{"CNAME mail.example.com", "mail", 2},          // substring
		{"TXT _dmarc.example.com v=spf1", "dmarc", 2}, // substring
		{"A cdn.example.com 1.2.3.4", "cdn", 2},       // substring
		{"MX mail.example.com", "zzz", -1},             // no match
	}
	for _, tt := range tests {
		got := fuzzyMatch(tt.text, tt.pattern)
		if got != tt.want {
			t.Errorf("fuzzyMatch(%q, %q) = %d, want %d", tt.text, tt.pattern, got, tt.want)
		}
	}
}

func TestMatchesWordBoundary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text    string
		pattern string
		want    bool
	}{
		{"cdn.example.com", "example", true},
		{"cdn.example.com", "cdn", true},
		{"cdn.example.com", "com", true},
		{"cdn.example.com", "ample", false},
		{"a-record test", "test", true},
		{"something", "xyz", false},
	}
	for _, tt := range tests {
		got := matchesWordBoundary(tt.text, tt.pattern)
		if got != tt.want {
			t.Errorf("matchesWordBoundary(%q, %q) = %v, want %v", tt.text, tt.pattern, got, tt.want)
		}
	}
}

// --- parseDNSRecords ---

func TestParseDNSRecords(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"status": "SUCCESS",
		"records": []any{
			map[string]any{
				"id":      "12345",
				"name":    "www.example.com",
				"type":    "A",
				"content": "1.2.3.4",
				"ttl":     "600",
				"prio":    "0",
				"notes":   "test note",
			},
			map[string]any{
				"id":      "12346",
				"name":    "example.com",
				"type":    "MX",
				"content": "mail.example.com",
				"ttl":     "3600",
				"prio":    "10",
			},
		},
	}
	records, err := parseDNSRecords(data)
	if err != nil {
		t.Fatalf("parseDNSRecords error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].ID != "12345" || records[0].Type != "A" || records[0].Content != "1.2.3.4" {
		t.Errorf("unexpected first record: %+v", records[0])
	}
	if records[0].Notes != "test note" {
		t.Errorf("expected notes='test note', got %q", records[0].Notes)
	}
	if records[1].Prio != "10" {
		t.Errorf("expected prio='10', got %q", records[1].Prio)
	}
}

func TestParseDNSRecordsMissingField(t *testing.T) {
	t.Parallel()
	_, err := parseDNSRecords(map[string]any{"status": "SUCCESS"})
	if err == nil {
		t.Fatal("expected error for missing records field")
	}
}

// --- DNS integration test helpers ---

func dnsTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		switch {
		// Delete by ID
		case strings.HasPrefix(path, "/dns/delete/example.com/"):
			fmt.Fprintf(w, `{"status":"SUCCESS"}`)

		// Delete by name+type
		case strings.HasPrefix(path, "/dns/deleteByNameType/example.com/"):
			fmt.Fprintf(w, `{"status":"SUCCESS"}`)

		// Edit by ID
		case strings.HasPrefix(path, "/dns/edit/example.com/"):
			fmt.Fprintf(w, `{"status":"SUCCESS"}`)

		// Edit by name+type
		case strings.HasPrefix(path, "/dns/editByNameType/example.com/"):
			fmt.Fprintf(w, `{"status":"SUCCESS"}`)

		// Create
		case strings.HasPrefix(path, "/dns/create/example.com"):
			fmt.Fprintf(w, `{"status":"SUCCESS","id":"99999"}`)

		// Retrieve by name+type with subdomain
		case strings.HasPrefix(path, "/dns/retrieveByNameType/example.com/A/www"):
			fmt.Fprintf(w, `{"status":"SUCCESS","records":[{"id":"12345","name":"www.example.com","type":"A","content":"1.2.3.4","ttl":"600","prio":"0","notes":""}]}`)

		// Retrieve by name+type (no subdomain)
		case strings.HasPrefix(path, "/dns/retrieveByNameType/example.com/MX"):
			fmt.Fprintf(w, `{"status":"SUCCESS","records":[{"id":"12346","name":"example.com","type":"MX","content":"mail.example.com","ttl":"3600","prio":"10","notes":""}]}`)

		// Retrieve single record by ID
		case strings.HasPrefix(path, "/dns/retrieve/example.com/12345"):
			fmt.Fprintf(w, `{"status":"SUCCESS","records":[{"id":"12345","name":"www.example.com","type":"A","content":"1.2.3.4","ttl":"600","prio":"0","notes":"web server"}]}`)

		// Retrieve all records
		case strings.HasPrefix(path, "/dns/retrieve/example.com"):
			fmt.Fprintf(w, `{"status":"SUCCESS","records":[{"id":"12345","name":"www.example.com","type":"A","content":"1.2.3.4","ttl":"600","prio":"0","notes":""},{"id":"12346","name":"example.com","type":"MX","content":"mail.example.com","ttl":"3600","prio":"10","notes":""},{"id":"12347","name":"example.com","type":"TXT","content":"v=spf1 include:_spf.example.com ~all","ttl":"3600","prio":"0","notes":""}]}`)

		// Retrieve from empty domain
		case strings.HasPrefix(path, "/dns/retrieve/empty.com"):
			fmt.Fprintf(w, `{"status":"SUCCESS","records":[]}`)

		// Ping (for loadKeys to succeed)
		case strings.HasPrefix(path, "/ping"):
			fmt.Fprintf(w, `{"status":"SUCCESS","yourIp":"1.2.3.4"}`)

		default:
			http.Error(w, `{"status":"ERROR","message":"not found"}`, 404)
		}
	}))
}

func setupDNSTest(t *testing.T) *httptest.Server {
	t.Helper()
	ts := dnsTestServer(t)
	t.Setenv("PORKBUN_API_KEY", "test_pk")
	t.Setenv("PORKBUN_SECRET_KEY", "test_sk")
	origBase := apiBase
	apiBase = ts.URL
	t.Cleanup(func() { apiBase = origBase; ts.Close() })
	return ts
}

// --- DNS list tests ---

func TestRunDNSList(t *testing.T) {
	setupDNSTest(t)

	err := run([]string{"dns", "list", "example.com"})
	if err != nil {
		t.Fatalf("dns list: %v", err)
	}
}

func TestRunDNSListJSON(t *testing.T) {
	setupDNSTest(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := run([]string{"dns", "list", "--json", "example.com"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("dns list --json: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := string(buf[:n])

	var arr []map[string]any
	if err := json.Unmarshal([]byte(got), &arr); err != nil {
		t.Fatalf("expected JSON array, got: %s", got)
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3 records, got %d", len(arr))
	}
}

func TestRunDNSListByType(t *testing.T) {
	setupDNSTest(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := run([]string{"dns", "list", "--type", "A", "--name", "www", "--json", "example.com"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("dns list --type A --name www: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := string(buf[:n])

	var arr []map[string]any
	if err := json.Unmarshal([]byte(got), &arr); err != nil {
		t.Fatalf("expected JSON array, got: %s", got)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 record, got %d", len(arr))
	}
	if arr[0]["type"] != "A" {
		t.Errorf("expected type=A, got %v", arr[0]["type"])
	}
}

func TestRunDNSListFilter(t *testing.T) {
	setupDNSTest(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := run([]string{"dns", "list", "--filter", "mail", "--json", "example.com"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("dns list --filter mail: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := string(buf[:n])

	var arr []map[string]any
	if err := json.Unmarshal([]byte(got), &arr); err != nil {
		t.Fatalf("expected JSON array, got: %s", got)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 filtered record, got %d", len(arr))
	}
	if arr[0]["_match_rank"] == nil {
		t.Error("expected _match_rank in filtered JSON output")
	}
}

func TestRunDNSListIDOnly(t *testing.T) {
	setupDNSTest(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := run([]string{"dns", "list", "--id-only", "example.com"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("dns list --id-only: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := strings.TrimSpace(string(buf[:n]))
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 IDs, got %d: %q", len(lines), got)
	}
}

func TestRunDNSListFirst(t *testing.T) {
	setupDNSTest(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := run([]string{"dns", "list", "--first", "--json", "example.com"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("dns list --first --json: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := string(buf[:n])

	var obj map[string]any
	if err := json.Unmarshal([]byte(got), &obj); err != nil {
		t.Fatalf("expected JSON object (--first), got: %s", got)
	}
	if obj["id"] == nil {
		t.Error("expected 'id' field in single-object output")
	}
}

func TestRunDNSListEmpty(t *testing.T) {
	setupDNSTest(t)

	err := run([]string{"dns", "list", "empty.com"})
	if err == nil {
		t.Fatal("expected error for empty results")
	}
}

func TestRunDNSListNameRequiresType(t *testing.T) {
	setupDNSTest(t)

	err := run([]string{"dns", "list", "--name", "www", "example.com"})
	if err == nil {
		t.Fatal("expected error when --name used without --type")
	}
}

// --- DNS get tests ---

func TestRunDNSGet(t *testing.T) {
	setupDNSTest(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := run([]string{"dns", "get", "--id", "12345", "example.com"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("dns get: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := string(buf[:n])
	if !strings.Contains(got, "1.2.3.4") {
		t.Errorf("expected content in output, got: %s", got)
	}
}

func TestRunDNSGetJSON(t *testing.T) {
	setupDNSTest(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := run([]string{"dns", "get", "--id", "12345", "--json", "example.com"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("dns get --json: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := string(buf[:n])

	var obj map[string]any
	if err := json.Unmarshal([]byte(got), &obj); err != nil {
		t.Fatalf("expected JSON object, got: %s", got)
	}
	if obj["notes"] != "web server" {
		t.Errorf("expected notes='web server', got %v", obj["notes"])
	}
}

func TestRunDNSGetMissingID(t *testing.T) {
	setupDNSTest(t)

	err := run([]string{"dns", "get", "example.com"})
	if err == nil {
		t.Fatal("expected error when --id is missing")
	}
}

// --- DNS create tests ---

func TestRunDNSCreate(t *testing.T) {
	setupDNSTest(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := run([]string{"dns", "create", "--type", "A", "--content", "5.6.7.8", "example.com"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("dns create: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := string(buf[:n])
	if !strings.Contains(got, "CREATED: 99999") {
		t.Errorf("expected CREATED output, got: %s", got)
	}
}

func TestRunDNSCreateIDOnly(t *testing.T) {
	setupDNSTest(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := run([]string{"dns", "create", "--type", "TXT", "--content", "hello", "--name", "_test", "-q", "example.com"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("dns create -q: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := strings.TrimSpace(string(buf[:n]))
	if got != "99999" {
		t.Errorf("expected '99999', got %q", got)
	}
}

func TestRunDNSCreateMissingFlags(t *testing.T) {
	setupDNSTest(t)

	err := run([]string{"dns", "create", "example.com"})
	if err == nil {
		t.Fatal("expected error when --type and --content missing")
	}
}

// --- DNS edit tests ---

func TestRunDNSEditByID(t *testing.T) {
	setupDNSTest(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := run([]string{"dns", "edit", "--id", "12345", "--type", "A", "--content", "9.8.7.6", "example.com"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("dns edit by ID: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := string(buf[:n])
	if !strings.Contains(got, "EDITED: 12345") {
		t.Errorf("expected EDITED output, got: %s", got)
	}
}

func TestRunDNSEditByNameType(t *testing.T) {
	setupDNSTest(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := run([]string{"dns", "edit", "--type", "A", "--name", "www", "--content", "9.8.7.6", "--json", "example.com"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("dns edit by name+type: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := string(buf[:n])

	var obj map[string]any
	if err := json.Unmarshal([]byte(got), &obj); err != nil {
		t.Fatalf("expected JSON, got: %s", got)
	}
	if obj["edited"] != true {
		t.Errorf("expected edited=true, got %v", obj["edited"])
	}
}

func TestRunDNSEditMissingFlags(t *testing.T) {
	setupDNSTest(t)

	err := run([]string{"dns", "edit", "--content", "foo", "example.com"})
	if err == nil {
		t.Fatal("expected error when neither --id nor --type provided")
	}
}

// --- DNS delete tests ---

func TestRunDNSDeleteByID(t *testing.T) {
	setupDNSTest(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := run([]string{"dns", "delete", "--id", "12345", "example.com"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("dns delete by ID: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := string(buf[:n])
	if !strings.Contains(got, "DELETED: 12345") {
		t.Errorf("expected DELETED output, got: %s", got)
	}
}

func TestRunDNSDeleteByNameType(t *testing.T) {
	setupDNSTest(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := run([]string{"dns", "delete", "--type", "A", "--name", "www", "--json", "example.com"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("dns delete by name+type: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := string(buf[:n])

	var obj map[string]any
	if err := json.Unmarshal([]byte(got), &obj); err != nil {
		t.Fatalf("expected JSON, got: %s", got)
	}
	if obj["deleted"] != true {
		t.Errorf("expected deleted=true, got %v", obj["deleted"])
	}
}

func TestRunDNSDeleteMissingFlags(t *testing.T) {
	setupDNSTest(t)

	err := run([]string{"dns", "delete", "example.com"})
	if err == nil {
		t.Fatal("expected error when neither --id nor --type provided")
	}
}

// --- DNS help ---

func TestRunDNSHelp(t *testing.T) {
	t.Parallel()
	err := run([]string{"dns"})
	if err != nil {
		t.Fatalf("dns help: %v", err)
	}
}
