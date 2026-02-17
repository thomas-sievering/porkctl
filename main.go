package main

import (
	"bytes"
	"encoding/json"
	"errors"
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

type apiKeys struct {
	APIKey    string
	SecretKey string
}

type bulkResult struct {
	Domain    string
	Available bool
	Price     string
	Renewal   string
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "version":
		fmt.Println(version)
		return
	case "ping":
		err = runPing()
	case "check":
		if len(args) != 1 {
			fmt.Println("Usage: porkctl check <domain>")
			fmt.Println("Example: porkctl check clau.de")
			os.Exit(1)
		}
		err = runCheck(args[0])
	case "check-bulk":
		if len(args) == 0 {
			fmt.Println("Usage: porkctl check-bulk <domain1> <domain2> ...")
			os.Exit(1)
		}
		err = runCheckBulk(args)
	case "register":
		if len(args) != 1 {
			fmt.Println("Usage: porkctl register <domain>")
			os.Exit(1)
		}
		err = runRegister(args[0])
	case "pricing":
		err = runPricing()
	default:
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}
}

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
}

func runPing() error {
	data, err := apiPost("/ping", nil)
	if err != nil {
		return err
	}
	status := asString(data["status"])
	if strings.EqualFold(status, "SUCCESS") {
		fmt.Println("STATUS: OK")
		fmt.Printf("IP: %s\n", fallback(asString(data["yourIp"]), "unknown"))
		return nil
	}
	fmt.Println("STATUS: FAILED")
	fmt.Printf("MESSAGE: %s\n", fallback(asString(data["message"]), "Unknown error"))
	return nil
}

func runCheck(domain string) error {
	data, err := apiPost("/domain/checkDomain/"+domain, nil)
	if err != nil {
		fmt.Printf("DOMAIN: %s\n", domain)
		fmt.Printf("ERROR: %v\n", err)
		return nil
	}
	avail, price, renewal := parseCheckResponse(data)
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
	msg := asString(data["message"])
	if msg != "" && !avail {
		fmt.Printf("MESSAGE: %s\n", msg)
	}
	return nil
}

func runCheckBulk(domains []string) error {
	fmt.Printf("CHECKING: %d domains\n\n", len(domains))
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

	maxDomain := len("DOMAIN")
	for _, r := range results {
		if len(r.Domain) > maxDomain {
			maxDomain = len(r.Domain)
		}
	}
	header := fmt.Sprintf("%-*s  AVAIL  REG_PRICE  RENEWAL", maxDomain, "DOMAIN")
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", len(header)))

	availCount := 0
	for _, r := range results {
		avail := "no"
		if r.Available {
			avail = "YES"
			availCount++
		}
		fmt.Printf("%-*s  %-5s  %-9s  %s\n", maxDomain, r.Domain, avail, r.Price, r.Renewal)
	}

	fmt.Printf("\nSUMMARY: %d/%d available\n", availCount, len(results))
	return nil
}

func runRegister(domain string) error {
	checkData, err := apiPost("/domain/checkDomain/"+domain, nil)
	if err != nil {
		return err
	}
	avail, price, renewal := parseCheckResponse(checkData)

	fmt.Printf("DOMAIN: %s\n", domain)
	if !avail {
		fmt.Println("AVAILABLE: no")
		fmt.Println("Cannot register - domain is not available.")
		return errors.New("domain unavailable")
	}
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
		fmt.Println("REGISTERED: yes")
		fmt.Printf("MESSAGE: %s\n", fallback(asString(regData["message"]), "Domain registered successfully"))
		return nil
	}

	fmt.Println("REGISTERED: no")
	fmt.Printf("ERROR: %s\n", fallback(asString(regData["message"]), "Registration failed"))
	return errors.New("registration failed")
}

func runPricing() error {
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
		TLD      string
		Register string
		Renewal  string
		RegNum   float64
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

	resp, err := http.Post(apiBase+endpoint, "application/json", bytes.NewReader(b))
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
	resp, err := http.Get(apiBase + endpoint)
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

func loadKeys() (apiKeys, error) {
	candidates := []string{}
	if p := strings.TrimSpace(os.Getenv("PORKCTL_ENV_FILE")); p != "" {
		candidates = append(candidates, p)
	}
	candidates = append(candidates,
		filepath.Join("C:", "dev", "_env", "secrets", "porkbun.env"),
		filepath.Join(".", "porkbun.env"),
		filepath.Join(".", ".env"),
		filepath.Join("C:", "dev", "_skills", "porkbun", ".env"),
	)

	var content []byte
	var chosen string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			b, err := os.ReadFile(c)
			if err == nil {
				content = b
				chosen = c
				break
			}
		}
	}
	if len(content) == 0 {
		return apiKeys{}, errors.New("no env file found; set PORKCTL_ENV_FILE or create C:/dev/_env/secrets/porkbun.env")
	}

	vals := parseEnv(string(content))
	apiKey := vals["PORKBUN_API_KEY"]
	secretKey := vals["PORKBUN_SECRET_KEY"]
	if apiKey == "" || secretKey == "" {
		return apiKeys{}, fmt.Errorf("missing PORKBUN_API_KEY or PORKBUN_SECRET_KEY in %s", chosen)
	}

	return apiKeys{APIKey: apiKey, SecretKey: secretKey}, nil
}

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



