package scaffolder

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"grabman/internal/doctor"
	"grabman/internal/manifest"
	"grabman/internal/solver"
)

var rePrereqGrabVersion = regexp.MustCompile(`GRAB\s+v?(\d+\.\d+\.\d+)`)

// PrereqOptions tunes system prerequisite checks (injectable for deterministic unit tests).
type PrereqOptions struct {
	GrabBinary      string
	SkipGrabCheck   bool
	RequirePostgres bool
	LookPathFn      func(string) (string, error)
	RunCmdFn        func(name string, args ...string) ([]byte, error)
	DialTCPFn       func(addr string) error
}

// PrereqStatus summarizes system prerequisite verification results.
type PrereqStatus struct {
	GrabDetected        bool   `json:"grab_detected"`
	GrabBinary          string `json:"grab_binary,omitempty"`
	GrabVersion         string `json:"grab_version,omitempty"`
	GrabInstallHint     string `json:"grab_install_hint,omitempty"`
	PostgresRequired    bool   `json:"postgres_required"`
	PostgresDetected    bool   `json:"postgres_detected"`
	PostgresDetail      string `json:"postgres_detail,omitempty"`
	PostgresInstallHint string `json:"postgres_install_hint,omitempty"`
}

const defaultGrabInstallHint = "Install GRAB v1.1.0+ from https://github.com/urbanspress/grab.stable/releases (Windows: place grab.exe in C:\\Program Files\\GRAB\\; Linux: /usr/local/bin/grab) and ensure 'grab' is on PATH."
const defaultPostgresInstallHint = "PostgreSQL 18 was not detected locally. Install PostgreSQL on Windows ('winget install PostgreSQL.PostgreSQL'), Ubuntu/Debian ('sudo apt-get install -y postgresql postgresql-client'), or start a local container ('docker compose -f deploy/docker-compose.yml up -d postgres-18')."

// CheckPrerequisites verifies GRAB (>= 1.1.0) and, for CMS profiles, PostgreSQL availability.
func CheckPrerequisites(profile string, opts PrereqOptions) (PrereqStatus, error) {
	lookPath := opts.LookPathFn
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	runCmd := opts.RunCmdFn
	if runCmd == nil {
		runCmd = func(name string, args ...string) ([]byte, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		}
	}
	dialTCP := opts.DialTCPFn
	if dialTCP == nil {
		dialTCP = func(addr string) error {
			conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
			if err != nil {
				return err
			}
			_ = conn.Close()
			return nil
		}
	}

	status := PrereqStatus{}
	if !opts.SkipGrabCheck {
		grabBin := strings.TrimSpace(opts.GrabBinary)
		if grabBin == "" && opts.LookPathFn != nil {
			if p, err := lookPath("grab"); err == nil {
				grabBin = p
			}
		}
		if grabBin == "" {
			grabBin = doctor.LocateGrabBinary("")
		}
		if grabBin != "" {
			if st, statErr := os.Stat(grabBin); statErr != nil || st.IsDir() {
				grabBin = ""
			}
		}
		if grabBin == "" {
			status.GrabInstallHint = defaultGrabInstallHint
			return status, manifest.NewError(
				manifest.ErrManifestMalformed,
				"GRAB CLI (>= v1.1.0) was not found on PATH or standard install locations",
				defaultGrabInstallHint,
			)
		}
		out, err := runCmd(grabBin, "version")
		if err != nil {
			status.GrabBinary = grabBin
			status.GrabInstallHint = defaultGrabInstallHint
			return status, manifest.WrapError(
				manifest.ErrManifestMalformed,
				fmt.Sprintf("found GRAB at %s but 'grab version' failed", grabBin),
				defaultGrabInstallHint,
				err,
			)
		}
		m := rePrereqGrabVersion.FindStringSubmatch(string(out))
		if len(m) < 2 {
			status.GrabBinary = grabBin
			status.GrabInstallHint = defaultGrabInstallHint
			return status, manifest.NewError(
				manifest.ErrSemVerViolation,
				fmt.Sprintf("unable to detect GRAB semantic version from %s", grabBin),
				defaultGrabInstallHint,
			)
		}
		sv, err := solver.ParseSemVer(m[1])
		minGrab := solver.SemVer{Major: 1, Minor: 1, Patch: 0}
		if err != nil || sv.Compare(minGrab) < 0 || sv.Major >= 2 {
			status.GrabBinary = grabBin
			status.GrabVersion = m[1]
			status.GrabInstallHint = defaultGrabInstallHint
			return status, manifest.NewError(
				manifest.ErrSemVerViolation,
				fmt.Sprintf("GRAB v%s does not satisfy required >= v1.1.0 (< v2.0.0) baseline", m[1]),
				defaultGrabInstallHint,
			)
		}
		status.GrabDetected = true
		status.GrabBinary = grabBin
		status.GrabVersion = sv.String()
	}

	if profile == "cms-stack" || profile == "ai-stack" {
		status.PostgresRequired = true
		detail := detectPostgres(lookPath, runCmd, dialTCP)
		if detail != "" {
			status.PostgresDetected = true
			status.PostgresDetail = detail
		} else {
			status.PostgresInstallHint = defaultPostgresInstallHint
			if opts.RequirePostgres {
				return status, manifest.NewError(
					manifest.ErrManifestMalformed,
					"PostgreSQL is required for GrabLab CMS but was not detected",
					defaultPostgresInstallHint,
				)
			}
		}
	}

	return status, nil
}

func detectPostgres(
	lookPath func(string) (string, error),
	runCmd func(string, ...string) ([]byte, error),
	dialTCP func(string) error,
) string {
	if pgReady, err := lookPath("pg_isready"); err == nil && pgReady != "" {
		if out, err := runCmd(pgReady, "-h", "127.0.0.1", "-p", "5432", "-t", "2"); err == nil {
			return fmt.Sprintf("pg_isready accepting connections (%s)", strings.TrimSpace(string(out)))
		}
		return fmt.Sprintf("pg_isready installed (%s)", pgReady)
	}
	if psqlPath, err := lookPath("psql"); err == nil && psqlPath != "" {
		if out, err := runCmd(psqlPath, "--version"); err == nil {
			return strings.TrimSpace(string(out))
		}
		return fmt.Sprintf("psql client installed (%s)", psqlPath)
	}
	if err := dialTCP("127.0.0.1:5432"); err == nil {
		return "PostgreSQL TCP port 127.0.0.1:5432 reachable"
	}
	if dockerPath, err := lookPath("docker"); err == nil && dockerPath != "" {
		if out, err := runCmd(dockerPath, "ps", "--format", "{{.Names}} {{.Image}}"); err == nil {
			for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
				if strings.Contains(strings.ToLower(line), "postgres") {
					return "Docker service running: " + strings.TrimSpace(line)
				}
			}
		}
	}
	return ""
}

// RunInteractiveSurvey runs a Next.js / Composer-style prompt flow that automatically includes
// GRAB and Grex JS v1.0.0, prompts only for optional GrabLab CMS and Groxy AI companion choices,
// and gates system prerequisites cleanly.
func RunInteractiveSurvey(in io.Reader, out io.Writer, opts *InitOptions, prereqOpts PrereqOptions) (*PrereqStatus, error) {
	if opts == nil {
		return nil, manifest.NewError(manifest.ErrManifestMalformed, "init options cannot be nil", "Provide valid InitOptions.")
	}
	fmt.Fprintln(out, "grabman v1.1.0 — Interactive GRAB Project Setup")
	fmt.Fprintln(out, "Mandatory core layers automatically included: GRAB (>= 1.1.0) & Grex JS v1.0.0 (public/grex/)")

	status, err := CheckPrerequisites("app", prereqOpts)
	if err != nil {
		return &status, err
	}
	if status.GrabDetected {
		fmt.Fprintf(out, "  [OK] GRAB v%s verified (%s)\n\n", status.GrabVersion, status.GrabBinary)
	}

	reader := bufio.NewReader(in)
	if strings.TrimSpace(opts.ProjectName) == "" {
		defaultName := "my-grab-app"
		if trimmedDir := strings.TrimSpace(opts.TargetDir); trimmedDir != "" && trimmedDir != "." {
			if base := filepath.Base(trimmedDir); base != "" && base != "." && base != string(filepath.Separator) {
				defaultName = base
			}
		}
		fmt.Fprintf(out, "Project name (%s): ", defaultName)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			opts.ProjectName = defaultName
		} else {
			opts.ProjectName = line
		}
	}

	installCMS := promptYesNo(reader, out, "Install GrabLab CMS? (Y/n): ", true)
	installGroxy := promptYesNo(reader, out, "Install Groxy AI companion? (y/N): ", false)

	switch {
	case installGroxy:
		opts.Profile = "ai-stack"
		if installCMS {
			fmt.Fprintln(out, "  -> Selected full-stack profile: GrabLab CMS + Groxy AI companion (bundle: ai-stack)")
		} else {
			fmt.Fprintln(out, "  -> Selected profile: ai-stack (Groxy AI companion + GRAB + Grex JS)")
		}
	case installCMS:
		opts.Profile = "cms-stack"
		fmt.Fprintln(out, "  -> Selected profile: cms-stack (GrabLab CMS + Grex Theme Builder + GRAB + Grex JS)")
	default:
		opts.Profile = "app"
		fmt.Fprintln(out, "  -> Selected profile: app (GRAB + Grex JS v1.0.0)")
	}

	if installCMS {
		cmsStatus, cmsErr := CheckPrerequisites(opts.Profile, prereqOpts)
		status = cmsStatus
		if cmsErr != nil {
			return &status, cmsErr
		}
		if status.PostgresDetected {
			fmt.Fprintf(out, "  [OK] PostgreSQL detected (%s)\n", status.PostgresDetail)
		} else {
			fmt.Fprintln(out, "  [WARN] PostgreSQL was not detected locally.")
			fmt.Fprintf(out, "         %s\n", status.PostgresInstallHint)
			if !promptYesNo(reader, out, "Continue scaffolding with local SQLite defaults while you install PostgreSQL? (Y/n): ", true) {
				return &status, manifest.NewError(
					manifest.ErrManifestMalformed,
					"project initialization cancelled while PostgreSQL is not installed",
					status.PostgresInstallHint,
				)
			}
		}
	}

	return &status, nil
}

func promptYesNo(r *bufio.Reader, out io.Writer, prompt string, defaultYes bool) bool {
	fmt.Fprint(out, prompt)
	line, err := r.ReadString('\n')
	ans := strings.ToLower(strings.TrimSpace(line))
	if err != nil && ans == "" {
		return defaultYes
	}
	if ans == "" {
		return defaultYes
	}
	if ans == "y" || ans == "yes" {
		return true
	}
	if ans == "n" || ans == "no" {
		return false
	}
	return defaultYes
}
