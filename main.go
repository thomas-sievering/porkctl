package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const apiBase = "https://api.porkbun.com/api/json/v3"

var version = "dev"

const defaultHTTPTimeout = 30 * time.Second

var porkHTTPClient = &http.Client{Timeout: defaultHTTPTimeout}

type apiKeys struct {
	APIKey    string `json:"api_key"`
	SecretKey string `json:"secret_key"`
}

type bulkResult struct {
	Domain    string `json:"domain"`
	Available bool   `json:"available"`
	Price     string `json:"price"`
	Renewal   string `json:"renewal"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		printJSONError(err)
		os.Exit(1)
	}
}

func run(argv []string) error {
	if len(argv) == 0 || argv[0] == "help" {
		printUsage()
		return nil
	}

	cmd := argv[0]
	args := argv[1:]

	switch cmd {
	case "version":
		return runVersion(args)
	case "ping":
		return runPing(args)
	case "check":
		return runCheck(args)
	case "check-bulk":
		return runCheckBulk(args)
	case "register":
		return runRegister(args)
	case "pricing":
		return runPricing(args)
	case "auth":
		return runAuth(args)
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

// --- JSON helpers ---

func writeJSON(v any) error {
	pretty := strings.TrimSpace(os.Getenv("PORKCTL_JSON_PRETTY")) == "1"
	var (
		b   []byte
		err error
	)
	if pretty {
		b, err = json.MarshalIndent(v, "", "  ")
	} else {
		b, err = json.Marshal(v)
	}
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func printJSON(v any) error {
	envelope := strings.TrimSpace(os.Getenv("PORKCTL_JSON_ENVELOPE")) == "1"
	if envelope {
		return writeJSON(map[string]any{"ok": true, "data": v})
	}
	return writeJSON(v)
}

func printJSONError(err error) {
	_ = writeJSON(map[string]any{
		"ok": false,
		"error": map[string]any{
			"code":    "FATAL",
			"message": err.Error(),
		},
	})
}

// --- Usage ---

func printUsage() {
	fmt.Println("Porkbun Domain Tool")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  version                       Print version")
	fmt.Println("  ping                          Verify API keys work")
	fmt.Println("  check <domain>                Check single domain availability")
	fmt.Println("  check-bulk <d1> <d2> ...      Check multiple domains")
	fmt.Println("  register <domain>             Register a domain")
	fmt.Println("  pricing                       Show TLD pricing (cheapest 50)")
	fmt.Println("  auth setup                    Show credential setup instructions")
	fmt.Println("  auth login                    Save API credentials")
	fmt.Println("  auth status                   Check stored credentials")
	fmt.Println("  auth logout                   Remove stored credentials")
	fmt.Println()
	fmt.Println("Global flags:")
	fmt.Println("  --json                        Output JSON envelope")
}

// --- Subcommands ---

func runVersion(args []string) error {
	fs := flag.NewFlagSet("porkctl version", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *jsonFlag {
		return printJSON(map[string]any{"version": version})
	}
	fmt.Println(version)
	return nil
}

func runPing(args []string) error {
	fs := flag.NewFlagSet("porkctl ping", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	data, err := apiPost("/ping", nil)
	if err != nil {
		return err
	}
	status := asString(data["status"])
	if *jsonFlag {
		return printJSON(map[string]any{
			"status":  strings.EqualFold(status, "SUCCESS"),
			"ip":      fallback(asString(data["yourIp"]), "unknown"),
			"message": fallback(asString(data["message"]), "Unknown error"),
		})
	}
	if strings.EqualFold(status, "SUCCESS") {
		fmt.Println("STATUS: OK")
		fmt.Printf("IP: %s\n", fallback(asString(data["yourIp"]), "unknown"))
		return nil
	}
	fmt.Println("STATUS: FAILED")
	fmt.Printf("MESSAGE: %s\n", fallback(asString(data["message"]), "Unknown error"))
	return nil
}

func runCheck(args []string) error {
	fs := flag.NewFlagSet("porkctl check", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("Usage: porkctl check <domain>")
	}
	domain := fs.Arg(0)

	data, err := apiPost("/domain/checkDomain/"+domain, nil)
	if err != nil {
		return err
	}
	avail, price, renewal := parseCheckResponse(data)
	msg := asString(data["message"])
	if *jsonFlag {
		out := map[string]any{
			"domain":         domain,
			"available":      avail,
			"register_price": price,
			"renewal_price":  renewal,
		}
		if msg != "" {
			out["message"] = msg
		}
		return printJSON(out)
	}
	fmt.Printf("DOMAIN: %s\n", domain)
	if avail {
		fmt.Println("AVAILABLE: yes")
	} else {
		fmt.Println("AVAILABLE: no")
	}
	if price != "-" {
		fmt.Printf("REGISTER_PRICE: %s\n", price)
	}
	if renewal != "-" {
		fmt.Printf("RENEWAL_PRICE: %s\n", renewal)
	}
	if msg != "" && !avail {
		fmt.Printf("MESSAGE: %s\n", msg)
	}
	return nil
}

func runCheckBulk(args []string) error {
	fs := flag.NewFlagSet("porkctl check-bulk", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	domains := fs.Args()
	if len(domains) == 0 {
		return errors.New("Usage: porkctl check-bulk <domain1> <domain2> ...")
	}

	if !*jsonFlag {
		fmt.Printf("CHECKING: %d domains\n\n", len(domains))
	}
	results := make([]bulkResult, 0, len(domains))

	for i, d := range domains {
		data, err := apiPost("/domain/checkDomain/"+d, nil)
		if err != nil {
			results = append(results, bulkResult{Domain: d, Available: false, Price: "error", Renewal: "error"})
		} else {
			avail, price, renewal := parseCheckResponse(data)
			results = append(results, bulkResult{Domain: d, Available: avail, Price: price, Renewal: renewal})
		}
		if i < len(domains)-1 {
			time.Sleep(1200 * time.Millisecond)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Available != results[j].Available {
			return results[i].Available
		}
		return len(results[i].Domain) < len(results[j].Domain)
	})

	availCount := 0
	for _, r := range results {
		if r.Available {
			availCount++
		}
	}

	if *jsonFlag {
		return printJSON(map[string]any{
			"count":           len(results),
			"available_count": availCount,
			"results":         results,
		})
	}

	maxDomain := len("DOMAIN")
	for _, r := range results {
		if len(r.Domain) > maxDomain {
			maxDomain = len(r.Domain)
		}
	}
	header := fmt.Sprintf("%-*s  AVAIL  REG_PRICE  RENEWAL", maxDomain, "DOMAIN")
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", len(header)))
	for _, r := range results {
		avail := "no"
		if r.Available {
			avail = "YES"
		}
		fmt.Printf("%-*s  %-5s  %-9s  %s\n", maxDomain, r.Domain, avail, r.Price, r.Renewal)
	}

	fmt.Printf("\nSUMMARY: %d/%d available\n", availCount, len(results))
	return nil
}

func runRegister(args []string) error {
	fs := flag.NewFlagSet("porkctl register", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("Usage: porkctl register <domain>")
	}
	domain := fs.Arg(0)

	checkData, err := apiPost("/domain/checkDomain/"+domain, nil)
	if err != nil {
		return err
	}
	avail, price, renewal := parseCheckResponse(checkData)
	if !avail {
		if !*jsonFlag {
			fmt.Printf("DOMAIN: %s\n", domain)
			fmt.Println("AVAILABLE: no")
			fmt.Println("Cannot register - domain is not available.")
		}
		return errors.New("domain unavailable")
	}

	fmt.Printf("DOMAIN: %s\n", domain)
	fmt.Println("AVAILABLE: yes")
	if price != "-" {
		fmt.Printf("REGISTER_PRICE: %s\n", price)
	}
	if renewal != "-" {
		fmt.Printf("RENEWAL_PRICE: %s\n", renewal)
	}

	minDuration := 1.0
	if responseMap, ok := asMap(checkData["response"]); ok {
		if f, ok := asFloat(responseMap["minDuration"]); ok && f > 0 {
			minDuration = f
		}
	}

	priceFloat, err := strconv.ParseFloat(price, 64)
	if err != nil {
		return fmt.Errorf("invalid registration price %q", price)
	}
	costCents := int64(priceFloat*100*minDuration + 0.5)

	regData, err := apiPost("/domain/create/"+domain, map[string]any{
		"cost":         costCents,
		"agreeToTerms": "yes",
	})
	if err != nil {
		return err
	}

	if strings.EqualFold(asString(regData["status"]), "SUCCESS") {
		if *jsonFlag {
			return printJSON(map[string]any{
				"domain":         domain,
				"available":      true,
				"registered":     true,
				"register_price": price,
				"renewal_price":  renewal,
				"message":        fallback(asString(regData["message"]), "Domain registered successfully"),
			})
		}
		fmt.Println("REGISTERED: yes")
		fmt.Printf("MESSAGE: %s\n", fallback(asString(regData["message"]), "Domain registered successfully"))
		return nil
	}

	fmt.Println("REGISTERED: no")
	fmt.Printf("ERROR: %s\n", fallback(asString(regData["message"]), "Registration failed"))
	return errors.New(fallback(asString(regData["message"]), "registration failed"))
}

func runPricing(args []string) error {
	fs := flag.NewFlagSet("porkctl pricing", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	data, err := apiGet("/pricing/get")
	if err != nil {
		return err
	}
	if !strings.EqualFold(asString(data["status"]), "SUCCESS") {
		return fmt.Errorf("%s", fallback(asString(data["message"]), "failed to get pricing"))
	}

	pricing, ok := asMap(data["pricing"])
	if !ok {
		return errors.New("missing pricing data")
	}

	type row struct {
		TLD      string  `json:"tld"`
		Register string  `json:"register"`
		Renewal  string  `json:"renewal"`
		RegNum   float64 `json:"-"`
	}
	rows := make([]row, 0, len(pricing))
	for tld, v := range pricing {
		entry, ok := asMap(v)
		if !ok {
			continue
		}
		reg := asString(entry["registration"])
		renew := asString(entry["renewal"])
		regNum, _ := strconv.ParseFloat(reg, 64)
		rows = append(rows, row{TLD: tld, Register: reg, Renewal: renew, RegNum: regNum})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].RegNum < rows[j].RegNum })
	if len(rows) > 50 {
		rows = rows[:50]
	}
	if *jsonFlag {
		return printJSON(map[string]any{
			"count": len(rows),
			"rows":  rows,
		})
	}

	maxTLD := len("TLD")
	for _, r := range rows {
		if len(r.TLD) > maxTLD {
			maxTLD = len(r.TLD)
		}
	}

	fmt.Printf("%-*s  REGISTER  RENEWAL\n", maxTLD, "TLD")
	fmt.Println(strings.Repeat("-", maxTLD+22))
	for _, r := range rows {
		fmt.Printf("%-*s  %-8s  %s\n", maxTLD, r.TLD, r.Register, r.Renewal)
	}
	fmt.Printf("\n... showing %d cheapest TLDs\n", len(rows))
	return nil
}

// --- Auth subcommands ---

func runAuth(args []string) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Println("Auth commands:")
		fmt.Println("  auth setup    Show credential setup instructions")
		fmt.Println("  auth login    Save API credentials")
		fmt.Println("  auth status   Check stored credentials")
		fmt.Println("  auth logout   Remove stored credentials")
		return nil
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "setup":
		return runAuthSetup(rest)
	case "login":
		return runAuthLogin(rest)
	case "status":
		return runAuthStatus(rest)
	case "logout":
		return runAuthLogout(rest)
	default:
		return fmt.Errorf("unknown auth command %q", sub)
	}
}

func runAuthSetup(args []string) error {
	fs := flag.NewFlagSet("porkctl auth setup", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	configDir, err := configPath()
	if err != nil {
		return err
	}

	steps := []map[string]string{
		{"step": "1", "action": "Create an API key at https://porkbun.com/account/api"},
		{"step": "2", "action": "Enable API access for each domain at https://porkbun.com/account/domainsSpe498"},
		{"step": "3", "action": fmt.Sprintf("Run: porkctl auth login")},
		{"step": "4", "action": "Verify with: porkctl auth status"},
	}

	if *jsonFlag {
		return printJSON(map[string]any{
			"config_dir": configDir,
			"steps":      steps,
		})
	}

	fmt.Println("Porkbun API Credential Setup")
	fmt.Println()
	for _, s := range steps {
		fmt.Printf("  %s. %s\n", s["step"], s["action"])
	}
	fmt.Printf("\nCredentials will be stored in: %s\n", filepath.Join(configDir, "config.json"))
	return nil
}

func runAuthLogin(args []string) error {
	fs := flag.NewFlagSet("porkctl auth login", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("API Key: ")
	apiKey, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading api key: %w", err)
	}
	apiKey = strings.TrimSpace(apiKey)

	fmt.Print("Secret Key: ")
	secretKey, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading secret key: %w", err)
	}
	secretKey = strings.TrimSpace(secretKey)

	if apiKey == "" || secretKey == "" {
		return errors.New("both API key and secret key are required")
	}

	configDir, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	cfg := map[string]string{
		"api_key":    apiKey,
		"secret_key": secretKey,
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	cfgFile := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(cfgFile, b, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	if *jsonFlag {
		return printJSON(map[string]any{
			"saved":       true,
			"config_file": cfgFile,
		})
	}
	fmt.Printf("Credentials saved to %s\n", cfgFile)
	return nil
}

func runAuthStatus(args []string) error {
	fs := flag.NewFlagSet("porkctl auth status", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	keys, source, err := resolveKeys()
	if err != nil {
		if *jsonFlag {
			return printJSON(map[string]any{
				"authenticated": false,
				"message":       err.Error(),
			})
		}
		fmt.Println("STATUS: not configured")
		fmt.Printf("MESSAGE: %s\n", err)
		return nil
	}

	// Mask keys for display
	maskedAPI := maskKey(keys.APIKey)
	maskedSecret := maskKey(keys.SecretKey)

	if *jsonFlag {
		return printJSON(map[string]any{
			"authenticated": true,
			"source":        source,
			"api_key":       maskedAPI,
			"secret_key":    maskedSecret,
		})
	}
	fmt.Println("STATUS: configured")
	fmt.Printf("SOURCE: %s\n", source)
	fmt.Printf("API_KEY: %s\n", maskedAPI)
	fmt.Printf("SECRET_KEY: %s\n", maskedSecret)
	return nil
}

func runAuthLogout(args []string) error {
	fs := flag.NewFlagSet("porkctl auth logout", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	configDir, err := configPath()
	if err != nil {
		return err
	}
	cfgFile := filepath.Join(configDir, "config.json")

	if _, err := os.Stat(cfgFile); errors.Is(err, os.ErrNotExist) {
		if *jsonFlag {
			return printJSON(map[string]any{"removed": false, "message": "no config file found"})
		}
		fmt.Println("No config file found.")
		return nil
	}

	if err := os.Remove(cfgFile); err != nil {
		return fmt.Errorf("removing config: %w", err)
	}

	if *jsonFlag {
		return printJSON(map[string]any{"removed": true, "config_file": cfgFile})
	}
	fmt.Printf("Credentials removed from %s\n", cfgFile)
	return nil
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// --- Config / Keys ---

func configPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(base, "porkctl"), nil
}

func loadKeys() (apiKeys, error) {
	keys, _, err := resolveKeys()
	return keys, err
}

func resolveKeys() (apiKeys, string, error) {
	// 1. Environment variables (highest priority)
	if apiKey, secretKey := resolveKeysFromEnv(); apiKey != "" && secretKey != "" {
		return apiKeys{APIKey: apiKey, SecretKey: secretKey}, "environment", nil
	}

	// 2. Explicit env file path
	if p := strings.TrimSpace(os.Getenv("PORKCTL_ENV_FILE")); p != "" {
		keys, err := loadKeysFromFile(p)
		if err == nil {
			return keys, "env_file:" + p, nil
		}
	}

	// 3. Config file in os.UserConfigDir()
	configDir, err := configPath()
	if err == nil {
		cfgFile := filepath.Join(configDir, "config.json")
		if keys, err := loadKeysFromConfig(cfgFile); err == nil {
			return keys, "config:" + cfgFile, nil
		}
	}

	// 4. Local env files
	localCandidates := []string{
		filepath.Join(".", "porkbun.env"),
		filepath.Join(".", ".env"),
	}
	for _, c := range localCandidates {
		if keys, err := loadKeysFromFile(c); err == nil {
			return keys, "file:" + c, nil
		}
	}

	return apiKeys{}, "", errors.New("no credentials found; run 'porkctl auth setup' for instructions")
}

func loadKeysFromConfig(path string) (apiKeys, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return apiKeys{}, err
	}
	var cfg map[string]string
	if err := json.Unmarshal(b, &cfg); err != nil {
		return apiKeys{}, err
	}
	apiKey := cfg["api_key"]
	secretKey := cfg["secret_key"]
	if apiKey == "" || secretKey == "" {
		return apiKeys{}, errors.New("missing api_key or secret_key in config")
	}
	return apiKeys{APIKey: apiKey, SecretKey: secretKey}, nil
}

func loadKeysFromFile(path string) (apiKeys, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return apiKeys{}, err
	}
	vals := parseEnv(string(b))
	apiKey := vals["PORKBUN_API_KEY"]
	secretKey := vals["PORKBUN_SECRET_KEY"]
	if apiKey == "" || secretKey == "" {
		return apiKeys{}, fmt.Errorf("missing PORKBUN_API_KEY or PORKBUN_SECRET_KEY in %s", path)
	}
	return apiKeys{APIKey: apiKey, SecretKey: secretKey}, nil
}

func resolveKeysFromEnv() (string, string) {
	apiKey := firstNonEmpty(
		os.Getenv("PORKBUN_API_KEY"),
		os.Getenv("PORKCTL_API_KEY"),
	)
	secretKey := firstNonEmpty(
		os.Getenv("PORKBUN_SECRET_KEY"),
		os.Getenv("PORKCTL_SECRET_KEY"),
	)
	return strings.TrimSpace(apiKey), strings.TrimSpace(secretKey)
}

// --- API helpers ---

func apiPost(endpoint string, body map[string]any) (map[string]any, error) {
	keys, err := loadKeys()
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"apikey":       keys.APIKey,
		"secretapikey": keys.SecretKey,
	}
	for k, v := range body {
		payload[k] = v
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, apiBase+endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := porkHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := decodeResponseMap(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		msg := fallback(asString(data["message"]), resp.Status)
		return nil, errors.New(msg)
	}
	if !strings.EqualFold(asString(data["status"]), "SUCCESS") {
		msg := fallback(asString(data["message"]), "request failed")
		return nil, errors.New(msg)
	}
	return data, nil
}

func apiGet(endpoint string) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, apiBase+endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := porkHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	return decodeResponseMap(resp.Body)
}

func decodeResponseMap(r io.Reader) (map[string]any, error) {
	var m map[string]any
	dec := json.NewDecoder(r)
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func parseCheckResponse(data map[string]any) (bool, string, string) {
	response := data
	if r, ok := asMap(data["response"]); ok {
		response = r
	}

	statusOK := strings.EqualFold(asString(data["status"]), "SUCCESS")
	avail := false
	if v, ok := response["avail"]; ok {
		switch vv := v.(type) {
		case bool:
			avail = vv
		case string:
			avail = strings.EqualFold(vv, "yes") || strings.EqualFold(vv, "true")
		}
	}
	avail = statusOK && avail

	price := "-"
	if p := asString(response["price"]); p != "" {
		price = p
	} else if pr, ok := asMap(response["pricing"]); ok {
		if p := asString(pr["registration"]); p != "" {
			price = p
		}
	}

	renewal := "-"
	if add, ok := asMap(response["additional"]); ok {
		if ren, ok := asMap(add["renewal"]); ok {
			if p := asString(ren["price"]); p != "" {
				renewal = p
			}
		}
	}
	if renewal == "-" {
		if pr, ok := asMap(response["pricing"]); ok {
			if p := asString(pr["renewal"]); p != "" {
				renewal = p
			}
		}
	}

	return avail, price, renewal
}

// --- Env parsing ---

func parseEnv(content string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		eq := strings.Index(trimmed, "=")
		if eq == -1 {
			continue
		}
		key := strings.TrimSpace(trimmed[:eq])
		val := strings.TrimSpace(trimmed[eq+1:])
		out[key] = strings.Trim(val, "\"")
	}
	return out
}

// --- Type helpers ---

func asMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func asString(v any) string {
	switch vv := v.(type) {
	case string:
		return vv
	case float64:
		return strconv.FormatFloat(vv, 'f', -1, 64)
	case int:
		return strconv.Itoa(vv)
	case int64:
		return strconv.FormatInt(vv, 10)
	case bool:
		if vv {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func asFloat(v any) (float64, bool) {
	switch vv := v.(type) {
	case float64:
		return vv, true
	case int:
		return float64(vv), true
	case int64:
		return float64(vv), true
	case string:
		f, err := strconv.ParseFloat(vv, 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}

func fallback(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
