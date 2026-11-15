#!/usr/bin/env python3
"""
Build static CGO_ENABLED=0 grabman binaries, bundle source archive, collect
walkthrough PDF, and emit SHA256SUMS.txt for GitHub Release distribution.
"""

import hashlib
import os
import shutil
import subprocess
import sys
import zipfile
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
DIST_DIR = REPO_ROOT / "dist"


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as fh:
        while chunk := fh.read(65536):
            h.update(chunk)
    return h.hexdigest()


def build_static_binary(goos: str, goarch: str, output_name: str, version: str) -> Path:
    out_path = DIST_DIR / output_name
    env = os.environ.copy()
    env["CGO_ENABLED"] = "0"
    env["GOOS"] = goos
    env["GOARCH"] = goarch
    ldflags = f"-s -w -X main.Version={version}"
    cmd = [
        "go",
        "build",
        "-trimpath",
        "-ldflags",
        ldflags,
        "-o",
        str(out_path),
        "./cmd/grabman",
    ]
    proc = subprocess.run(cmd, cwd=REPO_ROOT, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    if proc.returncode != 0:
        raise RuntimeError(f"go build failed for {goos}/{goarch}:\n{proc.stderr}\n{proc.stdout}")
    return out_path


def collect_bundle_files():
    include_dirs = ["cmd", "internal", "tests", "specifications", "docs", "scripts"]
    include_files = [
        "go.mod",
        "AGENTS.md",
        "GRABMAN-EXPERT.md",
        "HANDOFF.md",
        "README.md",
        "CHANGELOG.md",
        "LICENSE",
    ]
    files = []

    for fname in include_files:
        p = REPO_ROOT / fname
        if p.exists() and p.is_file():
            files.append(p)

    for dname in include_dirs:
        dpath = REPO_ROOT / dname
        if not dpath.exists():
            continue
        for item in sorted(dpath.rglob("*")):
            if item.is_file() and "__pycache__" not in item.parts:
                files.append(item)

    return files


def main():
    version = (sys.argv[1] if len(sys.argv) > 1 else "0.1.0").lstrip("v")

    if DIST_DIR.exists():
        for old in DIST_DIR.iterdir():
            if old.is_file():
                old.unlink()
    DIST_DIR.mkdir(parents=True, exist_ok=True)

    win_bin = build_static_binary("windows", "amd64", "grabman-windows-amd64.exe", version)
    lin_bin = build_static_binary("linux", "amd64", "grabman-linux-amd64", version)

    bundle_path = DIST_DIR / f"grabman-v{version}-bundle.zip"
    files = collect_bundle_files()
    with zipfile.ZipFile(bundle_path, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        for file_path in files:
            rel = file_path.relative_to(REPO_ROOT).as_posix()
            zf.write(file_path, arcname=f"grabman-v{version}/{rel}")

    checksum_targets = [win_bin, lin_bin, bundle_path]
    pdf_path = REPO_ROOT / "docs" / "releases" / "pdf" / f"Grabman-v{version}-Guide.pdf"
    if not pdf_path.exists():
        raise FileNotFoundError(f"Expected walkthrough PDF missing: {pdf_path}")
    dist_pdf = DIST_DIR / pdf_path.name
    shutil.copyfile(pdf_path, dist_pdf)
    checksum_targets.append(dist_pdf)

    sums_lines = [f"{sha256_file(p)}  {p.name}" for p in checksum_targets]
    sums_path = DIST_DIR / "SHA256SUMS.txt"
    sums_path.write_text("\n".join(sums_lines) + "\n", encoding="utf-8")

    print(f"Packaged Grabman v{version} assets into {DIST_DIR} ({len(files)} source/doc files in bundle):")
    for line in sums_lines:
        print(f"  {line}")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"[ERROR] {exc}", file=sys.stderr)
        sys.exit(1)
