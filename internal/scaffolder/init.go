package scaffolder

import (
	"fmt"
	"os"
	"strings"

	"grabman/internal/manifest"
	"grabman/internal/sitedna"
	"grabman/internal/solver"
)

// InitOptions configures `grabman init`.
type InitOptions struct {
	TargetDir      string
	Profile        string
	ProjectName    string
	EdgeProxy      string
	FromSiteDNA    string
	Force          bool
	FixedTimestamp string
}

// InitResult summarizes files created by `grabman init`.
type InitResult struct {
	SchemaVersion    string   `json:"schema_version"`
	Workspace        string   `json:"workspace"`
	ProjectName      string   `json:"project_name"`
	Profile          string   `json:"profile"`
	EdgeProxy        string   `json:"edge_proxy"`
	HydratedSiteDNA  bool     `json:"hydrated_site_dna"`
	FilesWritten     []string `json:"files_written"`
	LockDigestSHA256 string   `json:"lock_digest_sha256"`
}

// Init scaffolds a complete, grab check-ready GRAB Ecosystem workspace with zero remote scripts.
func Init(opts InitOptions) (*InitResult, error) {
	profile := strings.TrimSpace(opts.Profile)
	if profile == "" {
		if strings.TrimSpace(opts.FromSiteDNA) != "" {
			profile = "ai-stack"
		} else {
			profile = "app"
		}
	}
	switch profile {
	case "app", "cms-stack", "ai-stack":
	default:
		return nil, manifest.NewError(
			manifest.ErrManifestMalformed,
			fmt.Sprintf("invalid profile %q (expected app, cms-stack, or ai-stack)", profile),
			"Choose --profile app, --profile cms-stack, or --profile ai-stack.",
		)
	}
	opts.Profile = profile

	edgeProxy := strings.TrimSpace(opts.EdgeProxy)
	if edgeProxy == "" {
		edgeProxy = "caddy"
	}
	switch edgeProxy {
	case "caddy", "nginx", "cloudflare-tunnel":
	default:
		return nil, manifest.NewError(
			manifest.ErrSecurityPolicy,
			fmt.Sprintf("unsupported edge_proxy %q (expected caddy, nginx, or cloudflare-tunnel)", edgeProxy),
			"Choose --edge-proxy caddy, nginx, or cloudflare-tunnel.",
		)
	}
	opts.EdgeProxy = edgeProxy

	// Validate external Site DNA before touching or creating the target workspace directory.
	var dnaBundle *sitedna.Bundle
	var dnaFormatted []byte
	if dnaInput := strings.TrimSpace(opts.FromSiteDNA); dnaInput != "" {
		safePath, err := manifest.ResolveExternalReadFile(dnaInput)
		if err != nil {
			return nil, err
		}
		rawDNA, err := manifest.ReadBoundedFile(safePath)
		if err != nil {
			return nil, manifest.WrapError(manifest.ErrSiteDNAInvalid, fmt.Sprintf("unable to read Site DNA file %q", dnaInput), "Check file path and permissions.", err)
		}
		dnaBundle, err = sitedna.ValidateBytes(rawDNA)
		if err != nil {
			return nil, err
		}
		dnaFormatted = append([]byte(nil), rawDNA...)
		if len(dnaFormatted) > 0 && dnaFormatted[len(dnaFormatted)-1] != '\n' {
			dnaFormatted = append(dnaFormatted, '\n')
		}
	}

	realRoot, err := manifest.InitWorkspaceRoot(opts.TargetDir)
	if err != nil {
		return nil, err
	}

	ctx := buildScaffoldContext(opts, dnaBundle)

	manifestBytes, err := renderManifest(ctx)
	if err != nil {
		return nil, err
	}
	platformBytes, err := renderPlatformConfig(ctx)
	if err != nil {
		return nil, err
	}
	dataBytes, err := renderDataConfig(ctx)
	if err != nil {
		return nil, err
	}

	type fileSpec struct {
		rel  string
		data []byte
		opts manifest.ResolveOptions
	}

	files := []fileSpec{
		{rel: "grabman.json", data: manifestBytes, opts: manifest.ResolveOptions{AllowMissing: true}},
		{rel: "grab.platform.json", data: platformBytes, opts: manifest.ResolveOptions{AllowMissing: true}},
		{rel: "grab.data.json", data: dataBytes, opts: manifest.ResolveOptions{AllowMissing: true}},
		{rel: "src/app.grab", data: renderGrabEntry(ctx), opts: manifest.ResolveOptions{AllowMissing: true}},
		{rel: "views/home.grab.html", data: renderHomeTemplate(ctx), opts: manifest.ResolveOptions{AllowMissing: true}},
		{rel: "public/assets/site.css", data: renderStylesheet(ctx), opts: manifest.ResolveOptions{AllowMissing: true}},
		{rel: "deploy/docker-compose.yml", data: renderDockerCompose(ctx), opts: manifest.ResolveOptions{AllowMissing: true}},
		{rel: "deploy/.env.example", data: renderEnvExample(ctx), opts: manifest.ResolveOptions{AllowMissing: true, AllowEnvExample: true}},
		{rel: ".gitignore", data: renderGitignore(), opts: manifest.ResolveOptions{AllowMissing: true}},
	}

	if ctx.Profile == "cms-stack" || ctx.Profile == "ai-stack" {
		themeBytes, err := renderThemeConfig(ctx)
		if err != nil {
			return nil, err
		}
		files = append(files, fileSpec{rel: "theme/theme.json", data: themeBytes, opts: manifest.ResolveOptions{AllowMissing: true}})
	}
	if ctx.FromSiteDNA {
		files = append(files, fileSpec{rel: "storage/site-dna.json", data: dnaFormatted, opts: manifest.ResolveOptions{AllowMissing: true}})
	}

	// Pre-flight confinement and non-overwrite checks before any write occurs.
	checkTargets := make([]string, 0, len(files)+1)
	for _, f := range files {
		abs, _, err := manifest.ResolveWorkspaceFile(realRoot, f.rel, f.opts)
		if err != nil {
			return nil, err
		}
		checkTargets = append(checkTargets, f.rel)
		if !opts.Force {
			if _, statErr := os.Stat(abs); statErr == nil {
				return nil, manifest.NewError(
					manifest.ErrManifestMalformed,
					fmt.Sprintf("refusing to overwrite existing %q without --force", f.rel),
					"Choose an empty directory or pass --force to replace scaffolded files.",
				)
			}
		}
	}
	lockAbs, _, err := manifest.ResolveWorkspaceFile(realRoot, "grabman.lock", manifest.ResolveOptions{AllowMissing: true})
	if err != nil {
		return nil, err
	}
	if !opts.Force {
		if _, statErr := os.Stat(lockAbs); statErr == nil {
			return nil, manifest.NewError(
				manifest.ErrManifestMalformed,
				"refusing to overwrite existing \"grabman.lock\" without --force",
				"Choose an empty directory or pass --force.",
			)
		}
	}
	_ = checkTargets

	written := make([]string, 0, len(files)+1)
	for _, f := range files {
		if err := manifest.WriteFileAtomic(realRoot, f.rel, f.data, f.opts); err != nil {
			return nil, err
		}
		written = append(written, f.rel)
	}

	solveRes, err := solver.Solve(realRoot, solver.SolveOptions{FixedTimestamp: opts.FixedTimestamp})
	if err != nil {
		return nil, err
	}
	written = append(written, "grabman.lock")

	return &InitResult{
		SchemaVersion:    manifest.SchemaVersionV100,
		Workspace:        realRoot,
		ProjectName:      ctx.ProjectName,
		Profile:          ctx.Profile,
		EdgeProxy:        ctx.EdgeProxy,
		HydratedSiteDNA:  ctx.FromSiteDNA,
		FilesWritten:     written,
		LockDigestSHA256: solveRes.LockDigestSHA256,
	}, nil
}
