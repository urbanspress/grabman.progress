package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"grabman/internal/doctor"
	"grabman/internal/manifest"
	"grabman/internal/scaffolder"
	"grabman/internal/sitedna"
	"grabman/internal/solver"
)

// Version identifies the release version (overridable via -ldflags "-X main.Version=...").
var Version = "0.1.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	sub := strings.TrimSpace(args[0])
	rest := args[1:]

	switch sub {
	case "version", "-v", "--version":
		return cmdVersion(rest, stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "init":
		return cmdInit(rest, stdout, stderr)
	case "doctor":
		return cmdDoctor(rest, stdout, stderr)
	case "solve":
		return cmdSolve(rest, stdout, stderr)
	case "dna":
		return cmdDNA(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Error: unknown command %q\n\n", sub)
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `grabman v`+Version+` — Post-1.0 GRAB Ecosystem Scaffolder, Doctor & Lock Solver

Usage:
  grabman init   [--dir <path>] [--profile app|cms-stack|ai-stack] [--name <name>] [--edge-proxy caddy|nginx|cloudflare-tunnel] [--from-site-dna <file.json>] [--force] [--json]
  grabman doctor [--dir <path>] [--strict] [--grab-bin <path>] [--json]
  grabman solve  [--dir <path>] [--verify] [--fixed-timestamp <RFC3339>] [--json]
  grabman dna validate <file.json> [--json]
  grabman dna diff <old.json> <new.json> [--json]
  grabman version [--json]
`)
}

func cmdVersion(args []string, stdout, stderr io.Writer) int {
	asJSON := false
	for _, a := range args {
		if a == "--json" {
			asJSON = true
		} else {
			fmt.Fprintf(stderr, "Error: unknown flag %q for version\n", a)
			return 2
		}
	}
	if asJSON {
		writeJSON(stdout, map[string]any{
			"name":           "grabman",
			"version":        Version,
			"schema_version": manifest.SchemaVersionV010,
			"lock_version":   manifest.LockVersionV1,
		})
		return 0
	}
	fmt.Fprintf(stdout, "grabman v%s\n", Version)
	return 0
}

func cmdInit(args []string, stdout, stderr io.Writer) int {
	reordered := reorderFlags(args, map[string]bool{
		"dir":             true,
		"profile":         true,
		"name":            true,
		"edge-proxy":      true,
		"from-site-dna":   true,
		"fixed-timestamp": true,
	})

	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := fs.String("dir", "", "Target project directory")
	profile := fs.String("profile", "", "Ecosystem profile (app, cms-stack, ai-stack)")
	name := fs.String("name", "", "Project display name")
	edgeProxy := fs.String("edge-proxy", "caddy", "Edge proxy (caddy, nginx, cloudflare-tunnel)")
	fromDNA := fs.String("from-site-dna", "", "Optional Groxy AI Site DNA JSON export to hydrate tokens & catalog")
	force := fs.Bool("force", false, "Overwrite existing scaffold files")
	fixedTS := fs.String("fixed-timestamp", "", "Deterministic UTC RFC3339 timestamp override for grabman.lock")
	asJSON := fs.Bool("json", false, "Output JSON")

	if err := fs.Parse(reordered); err != nil {
		return writeError(*asJSON, stdout, stderr, err, 2)
	}
	pos := fs.Args()
	targetDir := strings.TrimSpace(*dir)
	if len(pos) > 1 || (len(pos) == 1 && targetDir != "") {
		return writeError(*asJSON, stdout, stderr, fmt.Errorf("unexpected positional arguments: %v", pos), 2)
	}
	if len(pos) == 1 {
		targetDir = pos[0]
	}
	if targetDir == "" {
		targetDir = "."
	}

	res, err := scaffolder.Init(scaffolder.InitOptions{
		TargetDir:      targetDir,
		Profile:        *profile,
		ProjectName:    *name,
		EdgeProxy:      *edgeProxy,
		FromSiteDNA:    *fromDNA,
		Force:          *force,
		FixedTimestamp: *fixedTS,
	})
	if err != nil {
		return writeError(*asJSON, stdout, stderr, err, 1)
	}

	if *asJSON {
		writeJSON(stdout, res)
		return 0
	}

	fmt.Fprintf(stdout, "Scaffolded GRAB workspace at %s\n", res.Workspace)
	fmt.Fprintf(stdout, "  Project:  %s (profile: %s, edge proxy: %s)\n", res.ProjectName, res.Profile, res.EdgeProxy)
	if res.HydratedSiteDNA {
		fmt.Fprintf(stdout, "  Site DNA: hydrated tokens & catalog into storage/site-dna.json\n")
	}
	fmt.Fprintf(stdout, "  Lock SHA: %s\n", res.LockDigestSHA256)
	fmt.Fprintf(stdout, "  Files (%d):\n", len(res.FilesWritten))
	for _, f := range res.FilesWritten {
		fmt.Fprintf(stdout, "    - %s\n", f)
	}
	return 0
}

func cmdDoctor(args []string, stdout, stderr io.Writer) int {
	reordered := reorderFlags(args, map[string]bool{
		"dir":      true,
		"grab-bin": true,
	})

	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := fs.String("dir", "", "Target project directory")
	strict := fs.Bool("strict", false, "Treat warnings as errors")
	grabBin := fs.String("grab-bin", "", "Optional explicit path to GRAB CLI executable")
	asJSON := fs.Bool("json", false, "Output JSON report")

	if err := fs.Parse(reordered); err != nil {
		return writeError(*asJSON, stdout, stderr, err, 2)
	}
	pos := fs.Args()
	targetDir := strings.TrimSpace(*dir)
	if len(pos) > 1 || (len(pos) == 1 && targetDir != "") {
		return writeError(*asJSON, stdout, stderr, fmt.Errorf("unexpected positional arguments: %v", pos), 2)
	}
	if len(pos) == 1 {
		targetDir = pos[0]
	}
	if targetDir == "" {
		targetDir = "."
	}

	rep, err := doctor.Run(targetDir, doctor.Options{
		GrabBinary: *grabBin,
		Strict:     *strict,
	})
	if err != nil {
		return writeError(*asJSON, stdout, stderr, err, 1)
	}

	if *asJSON {
		raw, err := doctor.FormatJSON(rep)
		if err != nil {
			return writeError(true, stdout, stderr, err, 1)
		}
		_, _ = stdout.Write(raw)
	} else {
		_, _ = io.WriteString(stdout, doctor.FormatPlainEnglish(rep))
	}
	return rep.ExitCode(*strict)
}

func cmdSolve(args []string, stdout, stderr io.Writer) int {
	reordered := reorderFlags(args, map[string]bool{
		"dir":             true,
		"fixed-timestamp": true,
	})

	fs := flag.NewFlagSet("solve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := fs.String("dir", "", "Target project directory")
	verifyOnly := fs.Bool("verify", false, "Verify existing grabman.lock deterministically without modifying files")
	fixedTS := fs.String("fixed-timestamp", "", "Deterministic UTC RFC3339 timestamp override")
	asJSON := fs.Bool("json", false, "Output JSON")

	if err := fs.Parse(reordered); err != nil {
		return writeError(*asJSON, stdout, stderr, err, 2)
	}
	pos := fs.Args()
	targetDir := strings.TrimSpace(*dir)
	if len(pos) > 1 || (len(pos) == 1 && targetDir != "") {
		return writeError(*asJSON, stdout, stderr, fmt.Errorf("unexpected positional arguments: %v", pos), 2)
	}
	if len(pos) == 1 {
		targetDir = pos[0]
	}
	if targetDir == "" {
		targetDir = "."
	}

	opts := solver.SolveOptions{FixedTimestamp: *fixedTS}

	if *verifyOnly {
		res, err := solver.Verify(targetDir, opts)
		if err != nil {
			return writeError(*asJSON, stdout, stderr, err, 1)
		}
		if *asJSON {
			writeJSON(stdout, res)
			return 0
		}
		fmt.Fprintf(stdout, "grabman.lock verified cleanly across all %d ecosystem layers\n", len(solver.EcosystemMatrix()))
		fmt.Fprintf(stdout, "  Workspace:  %s\n", res.Workspace)
		fmt.Fprintf(stdout, "  Lock SHA:   %s\n", res.LockDigestSHA256)
		return 0
	}

	res, err := solver.Solve(targetDir, opts)
	if err != nil {
		return writeError(*asJSON, stdout, stderr, err, 1)
	}
	if *asJSON {
		writeJSON(stdout, res)
		return 0
	}
	fmt.Fprintf(stdout, "Solved deterministic 6-layer lockfile: %s\n", res.LockfilePath)
	fmt.Fprintf(stdout, "  Lock SHA:   %s\n", res.LockDigestSHA256)
	fmt.Fprintf(stdout, "  Layers (%d): ", len(res.Lockfile.Layers))
	first := true
	for _, spec := range solver.EcosystemMatrix() {
		if l, ok := res.Lockfile.Layers[spec.Key]; ok {
			if !first {
				fmt.Fprint(stdout, ", ")
			}
			fmt.Fprintf(stdout, "%s@%s", l.Layer, l.Version)
			first = false
		}
	}
	fmt.Fprintln(stdout)
	return 0
}

func cmdDNA(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: grabman dna <validate|diff> ...")
		return 2
	}
	sub := strings.TrimSpace(args[0])
	rest := reorderFlags(args[1:], map[string]bool{})

	switch sub {
	case "validate":
		fs := flag.NewFlagSet("dna validate", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(rest); err != nil {
			return writeError(*asJSON, stdout, stderr, err, 2)
		}
		if fs.NArg() != 1 {
			return writeError(*asJSON, stdout, stderr, fmt.Errorf("usage: grabman dna validate <file.json> [--json]"), 2)
		}
		bundle, err := sitedna.ValidateFile(fs.Arg(0))
		if err != nil {
			return writeError(*asJSON, stdout, stderr, err, 1)
		}
		if *asJSON {
			writeJSON(stdout, map[string]any{
				"ok":                        true,
				"schema_version":            bundle.SchemaVersion,
				"bundle":                    bundle.BundleName,
				"site_name":                 bundle.SiteName,
				"style_pack_id":             bundle.ActiveStylePackID,
				"style_pack_label":          bundle.ActiveStylePackLabel,
				"product_count":             len(bundle.Products),
				"redirect_count":            len(bundle.Redirects),
				"site_health_score":         bundle.SiteHealthScore,
				"overall_grade":             bundle.OverallGrade,
				"decimal_integrity_verified": true,
				"sha256":                    bundle.RawSHA256,
			})
			return 0
		}
		fmt.Fprintf(stdout, "Site DNA VALID (%s / schema %s)\n", bundle.BundleName, bundle.SchemaVersion)
		fmt.Fprintf(stdout, "  Site:       %s (%s)\n", bundle.SiteName, bundle.SiteURL)
		fmt.Fprintf(stdout, "  Style Pack: %s (%s)\n", bundle.ActiveStylePackLabel, bundle.ActiveStylePackID)
		fmt.Fprintf(stdout, "  Catalog:    %d product(s) verified with exact ADR-025c base-10 decimals\n", len(bundle.Products))
		fmt.Fprintf(stdout, "  Redirects:  %d safe redirect(s)\n", len(bundle.Redirects))
		fmt.Fprintf(stdout, "  Health:     %d (%s)\n", bundle.SiteHealthScore, bundle.OverallGrade)
		fmt.Fprintf(stdout, "  SHA-256:    %s\n", bundle.RawSHA256)
		return 0

	case "diff":
		fs := flag.NewFlagSet("dna diff", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(rest); err != nil {
			return writeError(*asJSON, stdout, stderr, err, 2)
		}
		if fs.NArg() != 2 {
			return writeError(*asJSON, stdout, stderr, fmt.Errorf("usage: grabman dna diff <old.json> <new.json> [--json]"), 2)
		}
		diffRep, err := sitedna.DiffFiles(fs.Arg(0), fs.Arg(1))
		if err != nil {
			return writeError(*asJSON, stdout, stderr, err, 1)
		}
		if *asJSON {
			writeJSON(stdout, diffRep)
			return 0
		}
		_, _ = io.WriteString(stdout, sitedna.FormatDiffText(diffRep))
		return 0

	default:
		fmt.Fprintf(stderr, "Error: unknown dna subcommand %q (expected validate or diff)\n", sub)
		return 2
	}
}

// reorderFlags places flag arguments (and values for valueFlags) ahead of positional arguments
// so users can pass flags like `--json` after filenames or paths comfortably.
func reorderFlags(args []string, valueFlags map[string]bool) []string {
	var flagsPart []string
	var posPart []string
	afterTerminator := false

	for i := 0; i < len(args); i++ {
		a := args[i]
		if afterTerminator {
			posPart = append(posPart, a)
			continue
		}
		if a == "--" {
			afterTerminator = true
			continue
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			flagsPart = append(flagsPart, a)
			if !strings.Contains(a, "=") {
				name := strings.TrimLeft(a, "-")
				if valueFlags[name] && i+1 < len(args) {
					i++
					flagsPart = append(flagsPart, args[i])
				}
			}
			continue
		}
		posPart = append(posPart, a)
	}
	if afterTerminator {
		posPart = append([]string{"--"}, posPart...)
	}
	return append(flagsPart, posPart...)
}

func writeJSON(w io.Writer, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(w, `{"ok":false,"error":%q}`+"\n", err.Error())
		return
	}
	_, _ = w.Write(buf.Bytes())
}

func writeError(asJSON bool, stdout, stderr io.Writer, err error, code int) int {
	diagCode := manifest.ExtractCode(err)
	var dErr *manifest.DiagError
	remediation := ""
	if errors.As(err, &dErr) {
		remediation = dErr.Remediation
	}
	if asJSON {
		payload := map[string]any{
			"ok":    false,
			"error": err.Error(),
		}
		if diagCode != "" {
			payload["code"] = string(diagCode)
		}
		if remediation != "" {
			payload["remediation"] = remediation
		}
		writeJSON(stdout, payload)
		return code
	}
	if diagCode != "" {
		fmt.Fprintf(stderr, "Error [%s]: %s\n", diagCode, err.Error())
	} else {
		fmt.Fprintf(stderr, "Error: %s\n", err.Error())
	}
	if remediation != "" {
		fmt.Fprintf(stderr, "Remediation: %s\n", remediation)
	}
	return code
}
