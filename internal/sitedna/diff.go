package sitedna

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// FieldChange represents a deterministic old -> new change for a named property.
type FieldChange struct {
	Name string `json:"name"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

// ProductDelta summarizes changes to an ADR-025c product entry.
type ProductDelta struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Path        string `json:"path"`
	SKU         string `json:"sku"`
	OldPrice    string `json:"old_price,omitempty"`
	NewPrice    string `json:"new_price,omitempty"`
	OldCurrency string `json:"old_currency,omitempty"`
	NewCurrency string `json:"new_currency,omitempty"`
	ChangeType  string `json:"change_type"` // "added" | "removed" | "modified"
}

// RedirectDelta summarizes changes to a 301 redirect rule.
type RedirectDelta struct {
	OldPath       string `json:"old_path"`
	OldTargetPath string `json:"old_target_path,omitempty"`
	NewTargetPath string `json:"new_target_path,omitempty"`
	ChangeType    string `json:"change_type"` // "added" | "removed" | "modified"
}

// WatchScoreDelta summarizes Nightly Watch site health score & grade changes.
type WatchScoreDelta struct {
	OldScore int64  `json:"old_score"`
	NewScore int64  `json:"new_score"`
	Delta    int64  `json:"delta"`
	OldGrade string `json:"old_grade"`
	NewGrade string `json:"new_grade"`
}

// DiffReport represents a deterministic pre-deploy Site DNA diff.
type DiffReport struct {
	SchemaVersion           string          `json:"schema_version"`
	HasChanges              bool            `json:"has_changes"`
	SiteName                FieldChange     `json:"site_name"`
	StylePack               FieldChange     `json:"active_style_pack"`
	TokensAdded             []FieldChange   `json:"tokens_added"`
	TokensRemoved           []FieldChange   `json:"tokens_removed"`
	TokensChanged           []FieldChange   `json:"tokens_changed"`
	ReusableClassesAdded    []string        `json:"reusable_classes_added"`
	ReusableClassesRemoved  []string        `json:"reusable_classes_removed"`
	ProductDeltas           []ProductDelta  `json:"product_deltas"`
	RedirectDeltas          []RedirectDelta `json:"redirect_deltas"`
	NightlyWatch            WatchScoreDelta `json:"nightly_watch"`
	PlainEnglishSummary     []string        `json:"plain_english_summary"`
}

// DiffFiles validates both Site DNA bundles (GRAB-E401 + ADR-025c) and computes a deterministic diff.
func DiffFiles(oldPath, newPath string) (*DiffReport, error) {
	oldBundle, err := ValidateFile(oldPath)
	if err != nil {
		return nil, err
	}
	newBundle, err := ValidateFile(newPath)
	if err != nil {
		return nil, err
	}
	return DiffBundles(oldBundle, newBundle), nil
}

// DiffBytes validates two raw JSON Site DNA payloads and computes a deterministic diff.
func DiffBytes(oldRaw, newRaw []byte) (*DiffReport, error) {
	oldBundle, err := ValidateBytes(oldRaw)
	if err != nil {
		return nil, err
	}
	newBundle, err := ValidateBytes(newRaw)
	if err != nil {
		return nil, err
	}
	return DiffBundles(oldBundle, newBundle), nil
}

// DiffBundles computes a deterministic structured diff between two validated Site DNA bundles.
func DiffBundles(oldB, newB *Bundle) *DiffReport {
	report := &DiffReport{
		SchemaVersion: "0.1.0",
		SiteName: FieldChange{
			Name: "site_name",
			Old:  oldB.SiteName,
			New:  newB.SiteName,
		},
		StylePack: FieldChange{
			Name: "active_style_pack",
			Old:  oldB.ActiveStylePackID,
			New:  newB.ActiveStylePackID,
		},
		TokensAdded:            []FieldChange{},
		TokensRemoved:          []FieldChange{},
		TokensChanged:          []FieldChange{},
		ReusableClassesAdded:   []string{},
		ReusableClassesRemoved: []string{},
		ProductDeltas:          []ProductDelta{},
		RedirectDeltas:         []RedirectDelta{},
		NightlyWatch: WatchScoreDelta{
			OldScore: oldB.SiteHealthScore,
			NewScore: newB.SiteHealthScore,
			Delta:    newB.SiteHealthScore - oldB.SiteHealthScore,
			OldGrade: oldB.OverallGrade,
			NewGrade: newB.OverallGrade,
		},
		PlainEnglishSummary: []string{},
	}

	// Token diffs (sorted)
	tokenKeys := unionSortedKeys(oldB.Tokens, newB.Tokens)
	for _, k := range tokenKeys {
		ov, oOK := oldB.Tokens[k]
		nv, nOK := newB.Tokens[k]
		switch {
		case !oOK && nOK:
			report.TokensAdded = append(report.TokensAdded, FieldChange{Name: k, New: nv})
		case oOK && !nOK:
			report.TokensRemoved = append(report.TokensRemoved, FieldChange{Name: k, Old: ov})
		case ov != nv:
			report.TokensChanged = append(report.TokensChanged, FieldChange{Name: k, Old: ov, New: nv})
		}
	}

	// Reusable class diffs (sorted)
	for _, c := range newB.ReusableClasses {
		if !slices.Contains(oldB.ReusableClasses, c) {
			report.ReusableClassesAdded = append(report.ReusableClassesAdded, c)
		}
	}
	for _, c := range oldB.ReusableClasses {
		if !slices.Contains(newB.ReusableClasses, c) {
			report.ReusableClassesRemoved = append(report.ReusableClassesRemoved, c)
		}
	}
	sort.Strings(report.ReusableClassesAdded)
	sort.Strings(report.ReusableClassesRemoved)

	// Product price & catalog diffs (exact ADR-025c strings)
	oldProdMap := indexProducts(oldB.Products)
	newProdMap := indexProducts(newB.Products)
	for _, k := range unionSortedKeysGeneric(oldProdMap, newProdMap) {
		op, oOK := oldProdMap[k]
		np, nOK := newProdMap[k]
		switch {
		case !oOK && nOK:
			report.ProductDeltas = append(report.ProductDeltas, ProductDelta{
				Key: k, Title: np.Title, Path: np.Path, SKU: np.SKU,
				NewPrice: np.Price, NewCurrency: np.Currency, ChangeType: "added",
			})
		case oOK && !nOK:
			report.ProductDeltas = append(report.ProductDeltas, ProductDelta{
				Key: k, Title: op.Title, Path: op.Path, SKU: op.SKU,
				OldPrice: op.Price, OldCurrency: op.Currency, ChangeType: "removed",
			})
		case op.Price != np.Price || op.Currency != np.Currency || op.Availability != np.Availability:
			report.ProductDeltas = append(report.ProductDeltas, ProductDelta{
				Key: k, Title: np.Title, Path: np.Path, SKU: np.SKU,
				OldPrice: op.Price, NewPrice: np.Price,
				OldCurrency: op.Currency, NewCurrency: np.Currency,
				ChangeType: "modified",
			})
		}
	}

	// 301 redirect diffs
	oldRedMap := indexRedirects(oldB.Redirects)
	newRedMap := indexRedirects(newB.Redirects)
	for _, k := range unionSortedKeysGeneric(oldRedMap, newRedMap) {
		or, oOK := oldRedMap[k]
		nr, nOK := newRedMap[k]
		switch {
		case !oOK && nOK:
			report.RedirectDeltas = append(report.RedirectDeltas, RedirectDelta{
				OldPath: k, NewTargetPath: nr.TargetPath, ChangeType: "added",
			})
		case oOK && !nOK:
			report.RedirectDeltas = append(report.RedirectDeltas, RedirectDelta{
				OldPath: k, OldTargetPath: or.TargetPath, ChangeType: "removed",
			})
		case or.TargetPath != nr.TargetPath || or.StatusCode != nr.StatusCode:
			report.RedirectDeltas = append(report.RedirectDeltas, RedirectDelta{
				OldPath: k, OldTargetPath: or.TargetPath, NewTargetPath: nr.TargetPath, ChangeType: "modified",
			})
		}
	}

	if oldB.SiteName != newB.SiteName {
		report.PlainEnglishSummary = append(report.PlainEnglishSummary, fmt.Sprintf("Site name changed: %q -> %q", oldB.SiteName, newB.SiteName))
	}
	if oldB.ActiveStylePackID != newB.ActiveStylePackID {
		report.PlainEnglishSummary = append(report.PlainEnglishSummary, fmt.Sprintf("Active Style Pack switched: %s -> %s", oldB.ActiveStylePackID, newB.ActiveStylePackID))
	}
	if len(report.TokensAdded)+len(report.TokensRemoved)+len(report.TokensChanged) > 0 {
		report.PlainEnglishSummary = append(report.PlainEnglishSummary, fmt.Sprintf("Design tokens: %d added, %d removed, %d changed", len(report.TokensAdded), len(report.TokensRemoved), len(report.TokensChanged)))
	}
	if len(report.ReusableClassesAdded)+len(report.ReusableClassesRemoved) > 0 {
		report.PlainEnglishSummary = append(report.PlainEnglishSummary, fmt.Sprintf("Reusable .grex-c-* classes: %d added, %d removed", len(report.ReusableClassesAdded), len(report.ReusableClassesRemoved)))
	}
	for _, pd := range report.ProductDeltas {
		switch pd.ChangeType {
		case "added":
			report.PlainEnglishSummary = append(report.PlainEnglishSummary, fmt.Sprintf("Product added: %s (%s) at price %q %s", pd.Title, pd.SKU, pd.NewPrice, pd.NewCurrency))
		case "removed":
			report.PlainEnglishSummary = append(report.PlainEnglishSummary, fmt.Sprintf("Product removed: %s (%s)", pd.Title, pd.SKU))
		case "modified":
			report.PlainEnglishSummary = append(report.PlainEnglishSummary, fmt.Sprintf("Product updated: %s (%s) price %q %s -> %q %s", pd.Title, pd.SKU, pd.OldPrice, pd.OldCurrency, pd.NewPrice, pd.NewCurrency))
		}
	}
	for _, rd := range report.RedirectDeltas {
		switch rd.ChangeType {
		case "added":
			report.PlainEnglishSummary = append(report.PlainEnglishSummary, fmt.Sprintf("301 redirect added: %s -> %s", rd.OldPath, rd.NewTargetPath))
		case "removed":
			report.PlainEnglishSummary = append(report.PlainEnglishSummary, fmt.Sprintf("301 redirect removed: %s (was -> %s)", rd.OldPath, rd.OldTargetPath))
		case "modified":
			report.PlainEnglishSummary = append(report.PlainEnglishSummary, fmt.Sprintf("301 redirect updated: %s (%s -> %s)", rd.OldPath, rd.OldTargetPath, rd.NewTargetPath))
		}
	}
	if report.NightlyWatch.Delta != 0 || report.NightlyWatch.OldGrade != report.NightlyWatch.NewGrade {
		report.PlainEnglishSummary = append(report.PlainEnglishSummary, fmt.Sprintf("Nightly Watch score: %d (%s) -> %d (%s) (%+d)", report.NightlyWatch.OldScore, report.NightlyWatch.OldGrade, report.NightlyWatch.NewScore, report.NightlyWatch.NewGrade, report.NightlyWatch.Delta))
	}

	report.HasChanges = len(report.PlainEnglishSummary) > 0
	if !report.HasChanges {
		report.PlainEnglishSummary = append(report.PlainEnglishSummary, "No functional Site DNA differences detected between the two snapshots.")
	}
	return report
}

// FormatDiffText formats a human-readable plain-English Site DNA diff summary.
func FormatDiffText(r *DiffReport) string {
	var b strings.Builder
	b.WriteString("Site DNA Diff Summary\n")
	b.WriteString("=====================\n")
	for _, line := range r.PlainEnglishSummary {
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func indexProducts(list []Product) map[string]Product {
	m := make(map[string]Product, len(list))
	for _, p := range list {
		key := p.Path + "::" + p.SKU
		m[key] = p
	}
	return m
}

func indexRedirects(list []Redirect) map[string]Redirect {
	m := make(map[string]Redirect, len(list))
	for _, r := range list {
		m[r.OldPath] = r
	}
	return m
}

func unionSortedKeys(a, b map[string]string) []string {
	set := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		set[k] = struct{}{}
	}
	for k := range b {
		set[k] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func unionSortedKeysGeneric[V any](a, b map[string]V) []string {
	set := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		set[k] = struct{}{}
	}
	for k := range b {
		set[k] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
