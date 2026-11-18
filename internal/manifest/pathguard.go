package manifest

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const MaxWorkspaceReadBytes = 10 * 1024 * 1024 // 10 MiB limit matching GRAB-E401

// ResolveOptions tunes path confinement behavior for specific trusted template outputs.
type ResolveOptions struct {
	AllowMissing    bool
	AllowEnvExample bool
}

// IsSensitiveSegment reports whether a path segment targets sensitive secrets or VCS state
// matching GRAB v1.1.0 GRAB-E401 path confinement rules.
func IsSensitiveSegment(seg string, allowEnvExample bool) bool {
	lower := strings.ToLower(strings.TrimSpace(seg))
	if lower == "" {
		return false
	}
	switch lower {
	case ".git", ".env", ".ssh", "id_rsa", "id_ed25519", "id_ecdsa", "id_dsa":
		return true
	}
	if strings.HasPrefix(lower, ".env.") {
		if allowEnvExample && lower == ".env.example" {
			return false
		}
		return true
	}
	if strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".key") ||
		strings.HasSuffix(lower, ".p12") || strings.HasSuffix(lower, ".pfx") {
		return true
	}
	return false
}

// HasBlockedSegments detects empty paths, null bytes, parent traversal (..), and sensitive
// secret/VCS segments across both `/` and `\` separators.
func HasBlockedSegments(rawPath string, allowEnvExample bool) bool {
	if rawPath == "" || strings.ContainsRune(rawPath, '\x00') {
		return true
	}
	segments := strings.FieldsFunc(rawPath, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	if len(segments) == 0 {
		return true
	}
	for _, seg := range segments {
		if seg == ".." || IsSensitiveSegment(seg, allowEnvExample) {
			return true
		}
	}
	return false
}

// EvalExistingSymlinks resolves symlinks for existing paths or the deepest existing ancestor
// when creating new files/directories under a confined root.
func EvalExistingSymlinks(absPath string) (string, error) {
	real, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return real, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(absPath)
	if parent == absPath {
		return absPath, nil
	}
	realParent, err := EvalExistingSymlinks(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(realParent, filepath.Base(absPath)), nil
}

// IsPathWithinRoot returns true iff target resolves inside root (or equals root) with no
// sensitive segments in the relative suffix.
func IsPathWithinRoot(target, root string, allowEnvExample bool) bool {
	if runtime.GOOS == "windows" {
		if !strings.EqualFold(filepath.VolumeName(target), filepath.VolumeName(root)) {
			return false
		}
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	if rel != "." {
		for _, seg := range strings.Split(rel, string(filepath.Separator)) {
			if seg == ".." || IsSensitiveSegment(seg, allowEnvExample) {
				return false
			}
		}
	}
	return true
}

func validateCommonRaw(raw string, allowEnvExample bool) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.ContainsRune(trimmed, '\x00') {
		return NewError(ErrPathConfinement, "GRAB-E401 parity: empty or null path is denied", "Provide a clean workspace-relative path.")
	}
	if strings.HasPrefix(trimmed, `\\`) || strings.HasPrefix(trimmed, "//") {
		return NewError(ErrPathConfinement, "GRAB-E401 parity: UNC or network paths are not permitted", "Use a local directory path inside your workspace.")
	}
	if HasBlockedSegments(trimmed, allowEnvExample) {
		return NewError(
			ErrPathConfinement,
			fmt.Sprintf("GRAB-E401 parity: access to sensitive or parent-traversal path %q is denied", trimmed),
			"Avoid '..', '.git', '.env', '.ssh', '*.key', '*.pem', 'id_rsa', or 'id_ed25519' paths.",
		)
	}
	return nil
}

// ResolveWorkspaceRoot canonicalizes an existing workspace directory and enforces GRAB-E401 rules.
func ResolveWorkspaceRoot(dir string) (string, error) {
	ws := strings.TrimSpace(dir)
	if ws == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", WrapError(ErrPathConfinement, "GRAB-E401 parity: unable to resolve current working directory", "Pass --dir explicitly.", err)
		}
		ws = wd
	} else {
		if err := validateCommonRaw(ws, false); err != nil {
			return "", err
		}
	}
	absWS, err := filepath.Abs(ws)
	if err != nil {
		return "", WrapError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: invalid workspace path %q", ws), "Provide a valid directory path.", err)
	}
	realWS, err := filepath.EvalSymlinks(absWS)
	if err != nil {
		return "", WrapError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: workspace %q does not exist", ws), "Create the directory or run 'grabman init --dir <path>'.", err)
	}
	st, err := os.Stat(realWS)
	if err != nil || !st.IsDir() {
		return "", NewError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: workspace %q must be an existing directory", ws), "Provide a directory path.")
	}
	return realWS, nil
}

// InitWorkspaceRoot validates and prepares (creating if absent) a target directory for `grabman init`.
func InitWorkspaceRoot(dir string) (string, error) {
	ws := strings.TrimSpace(dir)
	if ws == "" {
		return ResolveWorkspaceRoot("")
	}
	if err := validateCommonRaw(ws, false); err != nil {
		return "", err
	}
	absWS, err := filepath.Abs(ws)
	if err != nil {
		return "", WrapError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: invalid target directory %q", ws), "Provide a valid directory path.", err)
	}
	realWS, err := EvalExistingSymlinks(absWS)
	if err != nil {
		return "", WrapError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: target directory %q cannot be resolved", ws), "Verify parent directory permissions and symlink targets.", err)
	}
	// Ensure resolved segments after symlink resolution contain no blocked segments.
	for _, seg := range strings.FieldsFunc(realWS, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." || IsSensitiveSegment(seg, false) {
			return "", NewError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: target directory %q resolves into sensitive path segment %q", ws, seg), "Choose a clean project directory.")
		}
	}
	if st, statErr := os.Stat(realWS); statErr == nil {
		if !st.IsDir() {
			return "", NewError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: target %q exists and is not a directory", ws), "Specify a directory path.")
		}
		return realWS, nil
	} else if !os.IsNotExist(statErr) {
		return "", WrapError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: cannot access %q", ws), "Check filesystem permissions.", statErr)
	}
	if err := os.MkdirAll(realWS, 0o755); err != nil {
		return "", WrapError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: failed to create workspace %q", ws), "Check directory permissions.", err)
	}
	finalWS, err := filepath.EvalSymlinks(realWS)
	if err != nil {
		return "", WrapError(ErrPathConfinement, "GRAB-E401 parity: failed to verify created workspace root", "Check filesystem permissions.", err)
	}
	return finalWS, nil
}

// ResolveWorkspaceFile confines any file path strictly inside workspaceRoot, rejecting symlinks
// escaping the workspace root, parent traversal, and sensitive segments (GRAB-E401 parity).
func ResolveWorkspaceFile(workspaceRoot, rawPath string, opts ResolveOptions) (string, string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if err := validateCommonRaw(trimmed, opts.AllowEnvExample); err != nil {
		return "", "", err
	}
	realRoot, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		return "", "", WrapError(ErrPathConfinement, "GRAB-E401 parity: invalid workspace root", "Provide an existing workspace directory.", err)
	}
	candidate := trimmed
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(realRoot, candidate)
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", "", WrapError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: invalid path %q", rawPath), "Use a clean relative path.", err)
	}
	realTarget, err := EvalExistingSymlinks(absCandidate)
	if err != nil {
		return "", "", WrapError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: path %q cannot be resolved", rawPath), "Ensure symlinks stay inside the workspace.", err)
	}
	if !IsPathWithinRoot(realTarget, realRoot, opts.AllowEnvExample) || realTarget == realRoot {
		return "", "", NewError(
			ErrPathConfinement,
			fmt.Sprintf("GRAB-E401 parity: path %q escapes workspace root %q", rawPath, realRoot),
			"Confine all reads and writes strictly inside the project directory.",
		)
	}
	rel, err := filepath.Rel(realRoot, realTarget)
	if err != nil || rel == "." {
		return "", "", NewError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: invalid relative path %q", rawPath), "Specify a file inside the workspace.")
	}
	slashRel := filepath.ToSlash(rel)

	st, statErr := os.Stat(realTarget)
	if statErr != nil {
		if !opts.AllowMissing || !os.IsNotExist(statErr) {
			return "", "", WrapError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: file %q is not accessible", slashRel), "Verify the file exists and is readable.", statErr)
		}
	} else {
		if !st.Mode().IsRegular() {
			return "", "", NewError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: %q is not a regular file", slashRel), "Point to a regular file.")
		}
		if st.Size() > MaxWorkspaceReadBytes {
			return "", "", NewError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: %q exceeds 10 MiB read limit", slashRel), "Reduce bundle size below 10 MiB.")
		}
	}

	return realTarget, slashRel, nil
}

// ResolveWorkspaceDir confines an existing subdirectory strictly inside workspaceRoot (GRAB-E401 parity).
// Returns os.ErrNotExist (wrapped) if the directory does not exist, without treating directories as files.
func ResolveWorkspaceDir(workspaceRoot, rawPath string) (string, string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if err := validateCommonRaw(trimmed, false); err != nil {
		return "", "", err
	}
	realRoot, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		return "", "", WrapError(ErrPathConfinement, "GRAB-E401 parity: invalid workspace root", "Provide an existing workspace directory.", err)
	}
	candidate := trimmed
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(realRoot, candidate)
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", "", WrapError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: invalid directory path %q", rawPath), "Use a clean relative path.", err)
	}
	realTarget, err := EvalExistingSymlinks(absCandidate)
	if err != nil {
		return "", "", WrapError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: directory %q cannot be resolved", rawPath), "Ensure symlinks stay inside the workspace.", err)
	}
	if !IsPathWithinRoot(realTarget, realRoot, false) || realTarget == realRoot {
		return "", "", NewError(
			ErrPathConfinement,
			fmt.Sprintf("GRAB-E401 parity: path %q escapes workspace root %q", rawPath, realRoot),
			"Confine directory scans strictly inside the project directory.",
		)
	}
	rel, err := filepath.Rel(realRoot, realTarget)
	if err != nil || rel == "." {
		return "", "", NewError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: invalid relative path %q", rawPath), "Specify a subdirectory inside the workspace.")
	}
	st, statErr := os.Stat(realTarget)
	if statErr != nil {
		return "", "", statErr
	}
	if !st.IsDir() {
		return "", "", NewError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: %q is not a directory", filepath.ToSlash(rel)), "Point to a workspace directory.")
	}
	return realTarget, filepath.ToSlash(rel), nil
}

// ResolveExternalReadFile validates a user-supplied input path (e.g. `grabman dna validate <file>`)
// that may reside outside the current working directory while still enforcing GRAB-E401 sensitive-path,
// traversal, symlink-resolution, regular-file, and 10 MiB read-limit checks.
func ResolveExternalReadFile(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if err := validateCommonRaw(trimmed, false); err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", WrapError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: invalid file path %q", rawPath), "Provide a valid file path.", err)
	}
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", WrapError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: file %q does not exist or cannot be resolved", rawPath), "Check the file path and try again.", err)
	}
	for _, seg := range strings.FieldsFunc(realPath, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." || IsSensitiveSegment(seg, false) {
			return "", NewError(
				ErrPathConfinement,
				fmt.Sprintf("GRAB-E401 parity: target %q resolves into sensitive path segment %q", rawPath, seg),
				"Never pass .git, .env, .ssh, *.key, or *.pem files.",
			)
		}
	}
	st, err := os.Stat(realPath)
	if err != nil {
		return "", WrapError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: cannot stat %q", rawPath), "Ensure the file exists and is readable.", err)
	}
	if !st.Mode().IsRegular() {
		return "", NewError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: %q is not a regular file", rawPath), "Provide a regular JSON file.")
	}
	if st.Size() > MaxWorkspaceReadBytes {
		return "", NewError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: %q exceeds 10 MiB read limit", rawPath), "Provide a file smaller than 10 MiB.")
	}
	return realPath, nil
}

// ReadBoundedFile reads a verified regular file strictly capped at MaxWorkspaceReadBytes.
func ReadBoundedFile(absPath string) ([]byte, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, NewError(ErrPathConfinement, "GRAB-E401 parity: target is not a regular file", "Point to a regular file.")
	}
	if st.Size() > MaxWorkspaceReadBytes {
		return nil, NewError(ErrPathConfinement, "GRAB-E401 parity: file exceeds 10 MiB read limit", "Provide a smaller file.")
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxWorkspaceReadBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxWorkspaceReadBytes {
		return nil, NewError(ErrPathConfinement, "GRAB-E401 parity: file exceeds 10 MiB read limit", "Provide a smaller file.")
	}
	return data, nil
}

// WriteFileAtomic writes data safely inside a pre-confined workspace file path via a temp file + rename.
func WriteFileAtomic(workspaceRoot, relPath string, data []byte, opts ResolveOptions) error {
	opts.AllowMissing = true
	absTarget, _, err := ResolveWorkspaceFile(workspaceRoot, relPath, opts)
	if err != nil {
		return err
	}
	parent := filepath.Dir(absTarget)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	// Re-verify parent after creation to defeat symlink TOCTOU races.
	realRoot, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		return err
	}
	realTarget, err := EvalExistingSymlinks(absTarget)
	if err != nil {
		return err
	}
	if !IsPathWithinRoot(realTarget, realRoot, opts.AllowEnvExample) {
		return NewError(ErrPathConfinement, fmt.Sprintf("GRAB-E401 parity: write target %q escapes workspace", relPath), "Keep all writes inside the workspace.")
	}
	tmp, err := os.CreateTemp(parent, ".grabman-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, absTarget); err != nil {
		return err
	}
	cleanup = false
	return nil
}

// WriteWorkspaceDotEnv atomically writes `.env` at the workspace root with strict 0600 permissions
// while keeping general workspace path operations closed against `.env` access.
func WriteWorkspaceDotEnv(workspaceRoot string, data []byte) error {
	realRoot, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		return WrapError(ErrPathConfinement, "GRAB-E401 parity: invalid workspace root for .env generation", "Provide a valid workspace directory.", err)
	}
	dotEnvPath := filepath.Join(realRoot, ".env")
	realTarget, err := EvalExistingSymlinks(dotEnvPath)
	if err != nil {
		return WrapError(ErrPathConfinement, "GRAB-E401 parity: .env target cannot be resolved", "Ensure .env is not a symlink escaping workspace.", err)
	}
	if filepath.Dir(realTarget) != realRoot || filepath.Base(realTarget) != ".env" {
		return NewError(ErrPathConfinement, "GRAB-E401 parity: .env target escapes workspace root", "Remove escaping symlinks at .env.")
	}
	if st, statErr := os.Lstat(dotEnvPath); statErr == nil && st.Mode()&os.ModeSymlink != 0 {
		return NewError(ErrPathConfinement, "GRAB-E401 parity: refusing to overwrite symlinked .env", "Remove symlinked .env before initializing.")
	}
	tmp, err := os.CreateTemp(realRoot, ".grabman-env-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dotEnvPath); err != nil {
		return err
	}
	cleanup = false
	return nil
}
