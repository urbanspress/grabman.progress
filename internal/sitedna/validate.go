package sitedna

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"

	"grabman/internal/manifest"
)

var (
	reExactBase10Signed   = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	reExactBase10Unsigned = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	reIEEE754FloatDrift   = regexp.MustCompile(`\.[0-9]*(?:0{6,}[1-9][0-9]{0,4}|9{6,}[0-8][0-9]{0,4})$`)
	reForbiddenHTMLTags   = regexp.MustCompile(`(?i)<\s*/?\s*(div|script|iframe|object|embed|style)\b`)
	reInlineEventHandler  = regexp.MustCompile(`(?i)(?:^|[\s<"'/;])on[a-z]{2,24}\s*=`)
	reUnsafeURLScheme     = regexp.MustCompile(`(?i)(?:javascript|vbscript|data)\s*:`)
	reCSSExpression       = regexp.MustCompile(`(?i)expression\s*\(`)
	reMarkdownInlineCode  = regexp.MustCompile("`[^`\n]+`")
)

// Bundle captures a validated Groxy AI Site DNA bundle (`groxy-site-dna-v0.4.0` or `v1.0.0`).
type Bundle struct {
	SchemaVersion            string            `json:"schema_version"`
	BundleName               string            `json:"bundle_name"`
	ExportedAt               string            `json:"exported_at,omitempty"`
	ReadOnlySnapshot         bool              `json:"read_only_snapshot"`
	CompatibleConsumers      []string          `json:"compatible_consumers"`
	SiteName                 string            `json:"site_name"`
	SiteURL                  string            `json:"site_url"`
	ActiveStylePackID        string            `json:"active_style_pack_id"`
	ActiveStylePackLabel     string            `json:"active_style_pack_label"`
	Tokens                   map[string]string `json:"tokens"`
	ReusableClasses          []string          `json:"reusable_classes"`
	Products                 []Product         `json:"products"`
	Redirects                []Redirect        `json:"redirects"`
	Counts                   map[string]int64  `json:"counts"`
	SiteHealthScore          int64             `json:"site_health_score"`
	OverallGrade             string            `json:"overall_grade"`
	DecimalIntegrityVerified bool              `json:"decimal_integrity_verified"`
	RawSHA256                string            `json:"raw_sha256"`
}

// Product represents an ADR-025c exact-decimal product record inside Site DNA.
type Product struct {
	Title        string `json:"title"`
	Path         string `json:"path"`
	SKU          string `json:"sku"`
	Price        string `json:"price"`
	Currency     string `json:"currency"`
	Availability string `json:"availability"`
}

// Redirect represents a 301 redirect record inside Site DNA.
type Redirect struct {
	OldPath    string `json:"old_path"`
	TargetPath string `json:"target_path"`
	StatusCode int64  `json:"status_code"`
}

// ValidationReport is emitted by `grabman dna validate`.
type ValidationReport struct {
	SchemaVersion string  `json:"schema_version"`
	Valid         bool    `json:"valid"`
	File          string  `json:"file,omitempty"`
	Code          string  `json:"code,omitempty"`
	Bundle        *Bundle `json:"bundle,omitempty"`
	Error         string  `json:"error,omitempty"`
	Remediation   string  `json:"remediation,omitempty"`
}

// IsExactGrabWireString validates an ADR-025c base-10 wire string and rejects float drift/scientific notation.
func IsExactGrabWireString(s string, allowNegative bool) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" || trimmed != s {
		return false
	}
	if allowNegative {
		if !reExactBase10Signed.MatchString(trimmed) {
			return false
		}
	} else {
		if !reExactBase10Unsigned.MatchString(trimmed) {
			return false
		}
	}
	return !reIEEE754FloatDrift.MatchString(trimmed)
}

func validateDecimalWireField(val any, fieldPath string, allowNegative bool) (string, error) {
	switch v := val.(type) {
	case string:
		if !IsExactGrabWireString(v, allowNegative) {
			return "", manifest.NewError(
				manifest.ErrSiteDNAInvalid,
				fmt.Sprintf("ADR-025c violation at %s: %q is not an exact base-10 GrabDecimal/GrabInteger wire string (float drift, whitespace, or scientific notation rejected)", fieldPath, v),
				"Export prices and exact counts as exact base-10 strings (for example, \"49.99\" or \"10020\").",
			)
		}
		return v, nil
	case json.Number:
		return "", manifest.NewError(
			manifest.ErrSiteDNAInvalid,
			fmt.Sprintf("ADR-025c violation at %s: unquoted JSON number %s found where an exact base-10 wire string is required", fieldPath, v.String()),
			"Serialize GrabDecimal/GrabInteger prices and quotas as quoted JSON strings, never raw numbers.",
		)
	default:
		return "", manifest.NewError(
			manifest.ErrSiteDNAInvalid,
			fmt.Sprintf("ADR-025c violation at %s: expected exact base-10 wire string", fieldPath),
			"Provide an exact base-10 string value.",
		)
	}
}

// ValidateFile reads and validates a Site DNA JSON file using GRAB-E401 path confinement.
func ValidateFile(filePath string) (*Bundle, error) {
	realPath, err := manifest.ResolveExternalReadFile(filePath)
	if err != nil {
		return nil, err
	}
	raw, err := manifest.ReadBoundedFile(realPath)
	if err != nil {
		return nil, err
	}
	return ValidateBytes(raw)
}

// ValidateBytes validates raw Groxy AI Site DNA JSON bytes against groxy-site-dna-v0.4.0 / v1.0.0,
// ADR-025c base-10 decimal fidelity, and zero-<div>/XSS hygiene rules.
func ValidateBytes(raw []byte) (*Bundle, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var root any
	if err := dec.Decode(&root); err != nil {
		return nil, manifest.WrapError(
			manifest.ErrSiteDNAInvalid,
			fmt.Sprintf("malformed Site DNA JSON: %v", err),
			"Provide a valid JSON export from GET /api/groxy/site-dna/export.",
			err,
		)
	}
	// Reject trailing tokens after the root JSON value.
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, manifest.NewError(
			manifest.ErrSiteDNAInvalid,
			"malformed Site DNA JSON: unexpected trailing data after root object",
			"Ensure the Site DNA file contains a single valid JSON object.",
		)
	}

	obj, ok := root.(map[string]any)
	if !ok {
		return nil, manifest.NewError(manifest.ErrSiteDNAInvalid, "Site DNA root must be a JSON object", "Export Site DNA as a JSON object.")
	}

	schemaVer := getString(obj, "schema_version")
	bundleName := getString(obj, "bundle_name")
	if schemaVer != "0.4.0" && schemaVer != "1.0.0" {
		return nil, manifest.NewError(
			manifest.ErrSiteDNAInvalid,
			fmt.Sprintf("unsupported Site DNA schema_version %q (expected \"0.4.0\" or \"1.0.0\")", schemaVer),
			"Export a compatible groxy-site-dna-v0.4.0 or v1.0.0 bundle from Groxy AI.",
		)
	}
	if bundleName != "groxy-site-dna-v0.4.0" && bundleName != "groxy-site-dna-v1.0.0" {
		return nil, manifest.NewError(
			manifest.ErrSiteDNAInvalid,
			fmt.Sprintf("unsupported Site DNA bundle_name %q (expected \"groxy-site-dna-v0.4.0\" or \"groxy-site-dna-v1.0.0\")", bundleName),
			"Export Site DNA from Groxy AI GET /api/groxy/site-dna/export.",
		)
	}
	if !getBoolTrue(obj, "read_only_snapshot") {
		return nil, manifest.NewError(
			manifest.ErrSiteDNAInvalid,
			"Site DNA bundle must declare read_only_snapshot: true",
			"Ensure Site DNA exports remain strictly read-only snapshots.",
		)
	}
	if !getBoolTrue(obj, "decimal_integrity_verified") {
		return nil, manifest.NewError(
			manifest.ErrSiteDNAInvalid,
			"Site DNA bundle must declare decimal_integrity_verified: true (ADR-025c)",
			"Re-export Site DNA after verifying all GrabDecimal/GrabInteger values.",
		)
	}

	consumers, err := getStringSlice(obj, "compatible_consumers")
	if err != nil || !slices.Contains(consumers, "grabman") {
		return nil, manifest.NewError(
			manifest.ErrSiteDNAInvalid,
			"Site DNA compatible_consumers must include \"grabman\"",
			"Include \"grabman\" in compatible_consumers.",
		)
	}

	siteName := strings.TrimSpace(getString(obj, "site_name"))
	siteURL := strings.TrimSpace(getString(obj, "site_url"))
	if siteName == "" || siteURL == "" {
		return nil, manifest.NewError(
			manifest.ErrSiteDNAInvalid,
			"Site DNA must include non-empty site_name and site_url",
			"Populate site_name and site_url in the Site DNA export.",
		)
	}
	if err := validateHTTPURL(siteURL, "site_url"); err != nil {
		return nil, err
	}

	// Required top-level sections
	stylePackObj, ok := obj["active_style_pack"].(map[string]any)
	if !ok || strings.TrimSpace(getString(stylePackObj, "id")) == "" {
		return nil, manifest.NewError(
			manifest.ErrSiteDNAInvalid,
			"Site DNA must include active_style_pack with a non-empty id",
			"Select a valid Groxy AI Design Style Pack before exporting Site DNA.",
		)
	}
	designObj, ok := obj["design_system"].(map[string]any)
	if !ok {
		return nil, manifest.NewError(manifest.ErrSiteDNAInvalid, "Site DNA missing required design_system object", "Include design_system in the Site DNA export.")
	}
	siteCtxObj, ok := obj["site_context"].(map[string]any)
	if !ok {
		return nil, manifest.NewError(manifest.ErrSiteDNAInvalid, "Site DNA missing required site_context object", "Include site_context in the Site DNA export.")
	}
	digestObj, ok := obj["nightly_watch_digest"].(map[string]any)
	if !ok {
		return nil, manifest.NewError(manifest.ErrSiteDNAInvalid, "Site DNA missing required nightly_watch_digest object", "Include nightly_watch_digest in the Site DNA export.")
	}

	// Run deep XSS / zero-<div> / unsafe URL scan across the entire document.
	if err := scanSafetyRecursive(obj, "$", false); err != nil {
		return nil, err
	}

	// Run deep ADR-025c decimal wire string audit on pricing / quota / custom_fields structures.
	if err := scanDecimalWireFieldsRecursive(obj, "$"); err != nil {
		return nil, err
	}

	tokens := extractTokens(designObj, stylePackObj)
	reusableClasses := extractReusableClasses(designObj)
	products, err := extractProducts(siteCtxObj)
	if err != nil {
		return nil, err
	}
	redirects, err := extractRedirects(siteCtxObj)
	if err != nil {
		return nil, err
	}

	counts := map[string]int64{}
	if countsMap, ok := siteCtxObj["counts"].(map[string]any); ok {
		for k, v := range countsMap {
			if n, ok := toInt64(v); ok {
				counts[k] = n
			}
		}
	}

	healthScore, _ := toInt64(digestObj["site_health_score"])
	overallGrade := strings.TrimSpace(getString(digestObj, "overall_grade"))

	return &Bundle{
		SchemaVersion:            schemaVer,
		BundleName:               bundleName,
		ExportedAt:               getString(obj, "exported_at"),
		ReadOnlySnapshot:         true,
		CompatibleConsumers:      consumers,
		SiteName:                 siteName,
		SiteURL:                  siteURL,
		ActiveStylePackID:        strings.TrimSpace(getString(stylePackObj, "id")),
		ActiveStylePackLabel:     strings.TrimSpace(getString(stylePackObj, "label")),
		Tokens:                   tokens,
		ReusableClasses:          reusableClasses,
		Products:                 products,
		Redirects:                redirects,
		Counts:                   counts,
		SiteHealthScore:          healthScore,
		OverallGrade:             overallGrade,
		DecimalIntegrityVerified: true,
		RawSHA256:                manifest.SHA256Bytes(raw),
	}, nil
}

func scanSafetyRecursive(node any, path string, isMarkdownField bool) error {
	switch v := node.(type) {
	case string:
		checkText := v
		if isMarkdownField {
			// Ignore documentation literals wrapped in single backticks (e.g. GRAB-DESIGN-SYSTEM.md prose).
			checkText = reMarkdownInlineCode.ReplaceAllString(v, " ")
		}
		if m := reForbiddenHTMLTags.FindString(checkText); m != "" {
			return manifest.NewError(
				manifest.ErrSiteDNAInvalid,
				fmt.Sprintf("unsafe HTML/DOM tag %q detected at %s (wrapper <div> and script/iframe/style injections are forbidden)", m, path),
				"Strip wrapper <div> elements and HTML/script tags using semantic Grex elements.",
			)
		}
		if m := reInlineEventHandler.FindString(checkText); m != "" {
			return manifest.NewError(
				manifest.ErrSiteDNAInvalid,
				fmt.Sprintf("unsafe inline event handler %q detected at %s", strings.TrimSpace(m), path),
				"Remove inline on* handlers from Site DNA content.",
			)
		}
		if m := reUnsafeURLScheme.FindString(checkText); m != "" {
			return manifest.NewError(
				manifest.ErrSiteDNAInvalid,
				fmt.Sprintf("unsafe URL scheme %q detected at %s", m, path),
				"Use only https://, http://, or relative / paths.",
			)
		}
		if reCSSExpression.MatchString(checkText) {
			return manifest.NewError(
				manifest.ErrSiteDNAInvalid,
				fmt.Sprintf("unsafe CSS expression() detected at %s", path),
				"Remove legacy CSS expression() values from design tokens.",
			)
		}
		return nil
	case []any:
		for i, item := range v {
			if err := scanSafetyRecursive(item, fmt.Sprintf("%s[%d]", path, i), false); err != nil {
				return err
			}
		}
	case map[string]any:
		keys := sortedMapKeys(v)
		for _, k := range keys {
			if err := scanSafetyRecursive(k, fmt.Sprintf("%s.<key:%s>", path, k), false); err != nil {
				return err
			}
			childIsMD := (k == "raw_markdown" || k == "markdown") && (strings.HasSuffix(path, ".design_system") || strings.HasSuffix(path, ".site_context"))
			if err := scanSafetyRecursive(v[k], fmt.Sprintf("%s.%s", path, k), childIsMD); err != nil {
				return err
			}
		}
	}
	return nil
}

func isStrictWireDecimalKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "price", "sale_price", "regular_price", "featured_price", "compare_at_price", "unit_price", "amount", "total", "weight", "rating":
		return true
	default:
		return false
	}
}

func scanDecimalWireFieldsRecursive(node any, path string) error {
	switch v := node.(type) {
	case []any:
		for i, item := range v {
			if err := scanDecimalWireFieldsRecursive(item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, k := range sortedMapKeys(v) {
			val := v[k]
			childPath := fmt.Sprintf("%s.%s", path, k)
			if isStrictWireDecimalKey(k) && val != nil && val != "" {
				if _, err := validateDecimalWireField(val, childPath, false); err != nil {
					return err
				}
				continue
			}
			if err := scanDecimalWireFieldsRecursive(val, childPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func extractTokens(designObj, stylePackObj map[string]any) map[string]string {
	tokens := make(map[string]string)
	if tokMap, ok := designObj["tokens"].(map[string]any); ok {
		for k, v := range tokMap {
			if s, ok := v.(string); ok {
				tokens[k] = s
			}
		}
	}
	if len(tokens) == 0 {
		if spTok, ok := stylePackObj["tokens"].(map[string]any); ok {
			if lightMap, ok := spTok["light"].(map[string]any); ok {
				for k, v := range lightMap {
					if s, ok := v.(string); ok {
						tokens[k] = s
					}
				}
			} else {
				for k, v := range spTok {
					if s, ok := v.(string); ok {
						tokens[k] = s
					}
				}
			}
		}
	}
	return tokens
}

func extractReusableClasses(designObj map[string]any) []string {
	rawList, ok := designObj["reusable_classes"].([]any)
	if !ok {
		return []string{}
	}
	set := make(map[string]struct{})
	for _, item := range rawList {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			set[strings.TrimSpace(s)] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func extractProducts(siteCtxObj map[string]any) ([]Product, error) {
	rawList, ok := siteCtxObj["products"].([]any)
	if !ok {
		return []Product{}, nil
	}
	out := make([]Product, 0, len(rawList))
	for idx, item := range rawList {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rawPrice, hasPrice := m["price"]
		if !hasPrice {
			return nil, manifest.NewError(
				manifest.ErrSiteDNAInvalid,
				fmt.Sprintf("ADR-025c violation: site_context.products[%d] is missing price", idx),
				"Include an exact base-10 string price on every product.",
			)
		}
		price, err := validateDecimalWireField(rawPrice, fmt.Sprintf("site_context.products[%d].price", idx), false)
		if err != nil {
			return nil, err
		}
		out = append(out, Product{
			Title:        strings.TrimSpace(getString(m, "title")),
			Path:         strings.TrimSpace(getString(m, "path")),
			SKU:          strings.TrimSpace(getString(m, "sku")),
			Price:        price,
			Currency:     strings.ToUpper(strings.TrimSpace(getString(m, "currency"))),
			Availability: strings.TrimSpace(getString(m, "availability")),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].SKU < out[j].SKU
	})
	return out, nil
}

func extractRedirects(siteCtxObj map[string]any) ([]Redirect, error) {
	rawList, ok := siteCtxObj["redirects"].([]any)
	if !ok {
		return []Redirect{}, nil
	}
	out := make([]Redirect, 0, len(rawList))
	for idx, item := range rawList {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		oldP := strings.TrimSpace(getString(m, "old_path"))
		targetP := strings.TrimSpace(getString(m, "target_path"))
		if oldP == "" || targetP == "" {
			return nil, manifest.NewError(
				manifest.ErrSiteDNAInvalid,
				fmt.Sprintf("site_context.redirects[%d] requires non-empty old_path and target_path", idx),
				"Fix invalid redirect entries before deploying.",
			)
		}
		statusCode, _ := toInt64(m["status_code"])
		if statusCode == 0 {
			statusCode = 301
		}
		out = append(out, Redirect{
			OldPath:    oldP,
			TargetPath: targetP,
			StatusCode: statusCode,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OldPath != out[j].OldPath {
			return out[i].OldPath < out[j].OldPath
		}
		return out[i].TargetPath < out[j].TargetPath
	})
	return out, nil
}

func validateHTTPURL(raw, fieldName string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return manifest.WrapError(manifest.ErrSiteDNAInvalid, fmt.Sprintf("invalid %s %q", fieldName, raw), "Use a valid https:// or http:// URL.", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && scheme != "http" {
		return manifest.NewError(manifest.ErrSiteDNAInvalid, fmt.Sprintf("unsafe %s scheme %q (expected http or https)", fieldName, u.Scheme), "Use an https:// or http:// URL.")
	}
	if strings.TrimSpace(u.Host) == "" {
		return manifest.NewError(manifest.ErrSiteDNAInvalid, fmt.Sprintf("%s %q must include a host", fieldName, raw), "Use a complete URL such as https://example.com.")
	}
	return nil
}

func getString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func getBoolTrue(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func getStringSlice(m map[string]any, key string) ([]string, error) {
	raw, ok := m[key].([]any)
	if !ok {
		return nil, fmt.Errorf("%s is not an array", key)
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s element is not a string", key)
		}
		out = append(out, s)
	}
	return out, nil
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
