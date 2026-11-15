package manifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	SchemaVersionV010 = "0.1.0"
	LockVersionV1     = "1"
)

// Frozen SHA-256 contract digests across the v1.x GRAB Ecosystem.
const (
	FrozenGrexElementsSHA256   = "c54c4bcf15ec8fea2dec4ef41975ba85a4b555dd400604e59c78883a8bbdebc9"
	FrozenGrexJSSHA256         = "c1e28d74a7ecf19182e225644fa244aa8600658dd1ff8895c133bfc64976b6d6"
	FrozenGrabLabGrexJSSHA256  = "eded264c31c3f5efa6e4b67ed06049eb5a5e517d34179efcbba921811059a377"
	FrozenSEOPlaybookV1SHA256  = "71817a8568908630b30c0d44d3319b3b86883d938b1d61466e78aad6d9d72cbe"
)

// Manifest represents author intent declared in `grabman.json`.
type Manifest struct {
	SchemaVersion  string            `json:"schema_version"`
	ProjectName    string            `json:"project_name"`
	Profile        string            `json:"profile"`
	TargetRuntime  map[string]string `json:"target_runtime"`
	SiteDNAPath    string            `json:"site_dna_path,omitempty"`
	DeploySecurity DeploySecurity    `json:"deploy_security"`
}

// DeploySecurity declares required production hardening defaults.
type DeploySecurity struct {
	ReadOnlyRoot      bool   `json:"read_only_root"`
	NonRootUID        string `json:"non_root_uid"`
	LoopbackOnlyPorts bool   `json:"loopback_only_ports"`
	EdgeProxy         string `json:"edge_proxy"`
}

// LockedLayer captures a resolved, pinned ecosystem layer inside `grabman.lock`.
type LockedLayer struct {
	Layer       string `json:"layer"`
	Repo        string `json:"repo"`
	Version     string `json:"version"`
	Constraint  string `json:"constraint"`
	BaselineMin string `json:"baseline_min"`
	SemVerTier  string `json:"semver_tier"`
}

// ContractDigest records a pinned SHA-256 contract digest in `grabman.lock`.
type ContractDigest struct {
	Artifact string `json:"artifact"`
	Source   string `json:"source"`
	SHA256   string `json:"sha256"`
}

// Lockfile represents deterministic output written to `grabman.lock`.
type Lockfile struct {
	LockVersion           string                    `json:"lock_version"`
	SchemaVersion         string                    `json:"schema_version"`
	ProjectName           string                    `json:"project_name"`
	Profile               string                    `json:"profile"`
	SolvedAtUTC           string                    `json:"solved_at_utc"`
	Layers                map[string]LockedLayer    `json:"layers"`
	Contracts             map[string]ContractDigest `json:"contracts"`
	CompatibilityVerified bool                      `json:"compatibility_verified"`
	LockDigestSHA256      string                    `json:"lock_digest_sha256"`
}

type lockCoreCanonical struct {
	LockVersion           string                    `json:"lock_version"`
	SchemaVersion         string                    `json:"schema_version"`
	ProjectName           string                    `json:"project_name"`
	Profile               string                    `json:"profile"`
	Layers                map[string]LockedLayer    `json:"layers"`
	Contracts             map[string]ContractDigest `json:"contracts"`
	CompatibilityVerified bool                      `json:"compatibility_verified"`
}

// ComputeLockDigestSHA256 returns the deterministic SHA-256 digest of the lockfile core,
// intentionally excluding `solved_at_utc` and `lock_digest_sha256` itself.
func (l *Lockfile) ComputeLockDigestSHA256() (string, error) {
	core := lockCoreCanonical{
		LockVersion:           l.LockVersion,
		SchemaVersion:         l.SchemaVersion,
		ProjectName:           l.ProjectName,
		Profile:               l.Profile,
		Layers:                l.Layers,
		Contracts:             l.Contracts,
		CompatibilityVerified: l.CompatibilityVerified,
	}
	b, err := json.Marshal(core)
	if err != nil {
		return "", err
	}
	return SHA256Bytes(b), nil
}

// CanonicalJSON renders deterministic formatted JSON with sorted map keys and a trailing newline.
func CanonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SHA256Bytes computes the lowercase hex SHA-256 of raw bytes.
func SHA256Bytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// SHA256File computes the lowercase hex SHA-256 of a regular bounded file.
func SHA256File(absPath string) (string, error) {
	data, err := ReadBoundedFile(absPath)
	if err != nil {
		return "", err
	}
	return SHA256Bytes(data), nil
}

// ValidateManifest checks structural integrity of `grabman.json`.
func ValidateManifest(m *Manifest) error {
	if m == nil {
		return NewError(ErrManifestMalformed, "grabman.json is empty", "Run 'grabman init' to generate a valid grabman.json.")
	}
	if strings.TrimSpace(m.SchemaVersion) != SchemaVersionV010 {
		return NewError(
			ErrManifestMalformed,
			fmt.Sprintf("unsupported grabman.json schema_version %q (expected %q)", m.SchemaVersion, SchemaVersionV010),
			fmt.Sprintf("Set schema_version to %q.", SchemaVersionV010),
		)
	}
	if strings.TrimSpace(m.ProjectName) == "" {
		return NewError(ErrManifestMalformed, "grabman.json project_name is required", "Set a non-empty project_name.")
	}
	switch strings.TrimSpace(m.Profile) {
	case "app", "cms-stack", "ai-stack":
	default:
		return NewError(
			ErrManifestMalformed,
			fmt.Sprintf("invalid grabman.json profile %q (expected app, cms-stack, or ai-stack)", m.Profile),
			"Choose profile: app, cms-stack, or ai-stack.",
		)
	}
	if len(m.TargetRuntime) == 0 {
		return NewError(ErrManifestMalformed, "grabman.json target_runtime constraints cannot be empty", "Declare SemVer ranges for the ecosystem layers.")
	}
	if m.DeploySecurity.EdgeProxy != "" {
		switch m.DeploySecurity.EdgeProxy {
		case "caddy", "nginx", "cloudflare-tunnel":
		default:
			return NewError(
				ErrSecurityPolicy,
				fmt.Sprintf("unsupported edge_proxy %q (expected caddy, nginx, or cloudflare-tunnel)", m.DeploySecurity.EdgeProxy),
				"Use caddy (recommended), nginx, or cloudflare-tunnel.",
			)
		}
	}
	return nil
}

// LoadManifest reads and validates `grabman.json` from inside workspaceRoot.
func LoadManifest(workspaceRoot string) (*Manifest, []byte, error) {
	abs, _, err := ResolveWorkspaceFile(workspaceRoot, "grabman.json", ResolveOptions{})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, WrapError(ErrManifestMalformed, "grabman.json is missing in workspace", "Run 'grabman init' first.", err)
		}
		return nil, nil, err
	}
	raw, err := ReadBoundedFile(abs)
	if err != nil {
		return nil, nil, WrapError(ErrManifestMalformed, "failed to read grabman.json", "Check file permissions.", err)
	}
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, nil, WrapError(ErrManifestMalformed, fmt.Sprintf("malformed grabman.json: %v", err), "Fix JSON syntax and schema fields in grabman.json.", err)
	}
	if err := ValidateManifest(&m); err != nil {
		return nil, nil, err
	}
	return &m, raw, nil
}

// LoadLockfile reads and parses `grabman.lock` from inside workspaceRoot.
func LoadLockfile(workspaceRoot string) (*Lockfile, []byte, error) {
	abs, _, err := ResolveWorkspaceFile(workspaceRoot, "grabman.lock", ResolveOptions{})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, WrapError(ErrManifestMalformed, "grabman.lock is missing in workspace", "Run 'grabman solve' to generate grabman.lock.", err)
		}
		return nil, nil, err
	}
	raw, err := ReadBoundedFile(abs)
	if err != nil {
		return nil, nil, WrapError(ErrManifestMalformed, "failed to read grabman.lock", "Run 'grabman solve' to regenerate grabman.lock.", err)
	}
	var l Lockfile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&l); err != nil {
		return nil, nil, WrapError(ErrManifestMalformed, fmt.Sprintf("malformed grabman.lock: %v", err), "Run 'grabman solve' to rebuild a clean lockfile.", err)
	}
	if l.LockVersion != LockVersionV1 {
		return nil, nil, NewError(ErrManifestMalformed, fmt.Sprintf("unsupported lock_version %q (expected %q)", l.LockVersion, LockVersionV1), "Regenerate grabman.lock with grabman solve.")
	}
	return &l, raw, nil
}
