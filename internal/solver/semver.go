package solver

import (
	"fmt"
	"strconv"
	"strings"

	"grabman/internal/manifest"
)

// SemVer represents a strict MAJOR.MINOR.PATCH version per ADR-027c.
type SemVer struct {
	Major uint64
	Minor uint64
	Patch uint64
}

// ParseSemVer parses a strict MAJOR.MINOR.PATCH string (optional leading "v").
func ParseSemVer(raw string) (SemVer, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	if s == "" {
		return SemVer{}, manifest.NewError(manifest.ErrSemVerViolation, "empty SemVer string", "Use strict MAJOR.MINOR.PATCH format (e.g., 1.1.0).")
	}
	if strings.ContainsAny(s, "*xX-+ \t") {
		return SemVer{}, manifest.NewError(manifest.ErrSemVerViolation, fmt.Sprintf("unpinned or non-standard SemVer %q rejected by ADR-027c", raw), "Specify an exact MAJOR.MINOR.PATCH version.")
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return SemVer{}, manifest.NewError(manifest.ErrSemVerViolation, fmt.Sprintf("invalid SemVer %q: must have exactly MAJOR.MINOR.PATCH", raw), "Use three numeric components such as 1.1.0.")
	}
	maj, err := parseStrictUint(parts[0])
	if err != nil {
		return SemVer{}, manifest.WrapError(manifest.ErrSemVerViolation, fmt.Sprintf("invalid major version in %q", raw), "Use numeric MAJOR.MINOR.PATCH.", err)
	}
	min, err := parseStrictUint(parts[1])
	if err != nil {
		return SemVer{}, manifest.WrapError(manifest.ErrSemVerViolation, fmt.Sprintf("invalid minor version in %q", raw), "Use numeric MAJOR.MINOR.PATCH.", err)
	}
	pat, err := parseStrictUint(parts[2])
	if err != nil {
		return SemVer{}, manifest.WrapError(manifest.ErrSemVerViolation, fmt.Sprintf("invalid patch version in %q", raw), "Use numeric MAJOR.MINOR.PATCH.", err)
	}
	return SemVer{Major: maj, Minor: min, Patch: pat}, nil
}

func parseStrictUint(part string) (uint64, error) {
	if part == "" {
		return 0, fmt.Errorf("empty version component")
	}
	if len(part) > 1 && part[0] == '0' {
		return 0, fmt.Errorf("leading zero in %q", part)
	}
	return strconv.ParseUint(part, 10, 64)
}

// String formats SemVer canonically as MAJOR.MINOR.PATCH.
func (v SemVer) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare returns -1 if v < other, 0 if v == other, and +1 if v > other.
func (v SemVer) Compare(other SemVer) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}
	return 0
}

type comparator struct {
	op  string
	ver SemVer
}

// ConstraintSet is a conjunction of SemVer comparators.
type ConstraintSet struct {
	Raw         string
	Comparators []comparator
}

// ParseConstraint parses SemVer bounds (`>=1.1.0 <2.0.0`, `^1.0.1`, `~1.0.1`, `=1.1.0`).
// Rejects unpinned wildcards (`*`, `latest`) and unauthorized `>= 2.0.0` major jumps per ADR-027c.
func ParseConstraint(expr string) (ConstraintSet, error) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return ConstraintSet{}, manifest.NewError(manifest.ErrSemVerViolation, "empty SemVer constraint", "Provide a pinned SemVer range such as >=1.1.0 <2.0.0.")
	}
	lower := strings.ToLower(trimmed)
	if lower == "*" || lower == "latest" || strings.Contains(lower, "latest") || strings.Contains(lower, "*") {
		return ConstraintSet{}, manifest.NewError(
			manifest.ErrSemVerViolation,
			fmt.Sprintf("unpinned constraint %q rejected by ADR-027c", expr),
			"Replace wildcard/latest ranges with pinned SemVer bounds (e.g., >=1.1.0 <2.0.0).",
		)
	}

	normalized := strings.ReplaceAll(trimmed, ",", " ")
	tokens := normalizeTokens(strings.Fields(normalized))
	if len(tokens) == 0 {
		return ConstraintSet{}, manifest.NewError(manifest.ErrSemVerViolation, fmt.Sprintf("empty constraint %q", expr), "Provide valid SemVer bounds.")
	}

	var comps []comparator
	for _, tok := range tokens {
		expanded, err := expandToken(tok)
		if err != nil {
			return ConstraintSet{}, err
		}
		comps = append(comps, expanded...)
	}

	for _, c := range comps {
		if c.ver.Major >= 2 && (c.op == ">=" || c.op == ">" || c.op == "=") {
			return ConstraintSet{}, manifest.NewError(
				manifest.ErrSemVerViolation,
				fmt.Sprintf("ADR-027c violation: constraint %q attempts an unapproved v2.0.0+ major boundary jump", expr),
				"Keep ecosystem targets strictly on the v1.x/v0.x contract line (<2.0.0) unless the project owner explicitly authorizes v2.0.0.",
			)
		}
	}

	return ConstraintSet{Raw: trimmed, Comparators: comps}, nil
}

func normalizeTokens(fields []string) []string {
	var out []string
	for i := 0; i < len(fields); i++ {
		tok := fields[i]
		if (tok == ">=" || tok == "<=" || tok == ">" || tok == "<" || tok == "=" || tok == "^" || tok == "~") && i+1 < len(fields) {
			out = append(out, tok+fields[i+1])
			i++
			continue
		}
		out = append(out, tok)
	}
	return out
}

func expandToken(tok string) ([]comparator, error) {
	if strings.HasPrefix(tok, "^") {
		base, err := ParseSemVer(strings.TrimPrefix(tok, "^"))
		if err != nil {
			return nil, err
		}
		var upper SemVer
		switch {
		case base.Major > 0:
			upper = SemVer{Major: base.Major + 1, Minor: 0, Patch: 0}
		case base.Minor > 0:
			upper = SemVer{Major: 0, Minor: base.Minor + 1, Patch: 0}
		default:
			upper = SemVer{Major: 0, Minor: 0, Patch: base.Patch + 1}
		}
		return []comparator{
			{op: ">=", ver: base},
			{op: "<", ver: upper},
		}, nil
	}

	if strings.HasPrefix(tok, "~") {
		base, err := ParseSemVer(strings.TrimPrefix(tok, "~"))
		if err != nil {
			return nil, err
		}
		upper := SemVer{Major: base.Major, Minor: base.Minor + 1, Patch: 0}
		return []comparator{
			{op: ">=", ver: base},
			{op: "<", ver: upper},
		}, nil
	}

	for _, op := range []string{">=", "<=", ">", "<", "="} {
		if strings.HasPrefix(tok, op) {
			v, err := ParseSemVer(strings.TrimPrefix(tok, op))
			if err != nil {
				return nil, err
			}
			return []comparator{{op: op, ver: v}}, nil
		}
	}

	v, err := ParseSemVer(tok)
	if err != nil {
		return nil, err
	}
	return []comparator{{op: "=", ver: v}}, nil
}

// Matches returns true iff v satisfies every comparator in the ConstraintSet.
func (cs ConstraintSet) Matches(v SemVer) bool {
	for _, c := range cs.Comparators {
		cmp := v.Compare(c.ver)
		switch c.op {
		case "=":
			if cmp != 0 {
				return false
			}
		case ">=":
			if cmp < 0 {
				return false
			}
		case "<=":
			if cmp > 0 {
				return false
			}
		case ">":
			if cmp <= 0 {
				return false
			}
		case "<":
			if cmp >= 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
