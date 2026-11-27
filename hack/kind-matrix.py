#!/usr/bin/env python3
"""Resolve published image aliases once, including a shared digest for latest."""

import json
import os
import re
import subprocess


def release_lines(releases):
    lines = set()
    for release in releases:
        if release.get("draft") or release.get("prerelease"):
            continue
        match = re.fullmatch(r"v(\d+)\.(\d+)\.\d+", release["tag_name"])
        if match:
            lines.add(tuple(map(int, match.groups())))
    if len(lines) < 2:
        raise ValueError("expected at least two published stable minor release lines")
    return [f"{major}.{minor}.z" for major, minor in sorted(lines, reverse=True)[:2]]


def build_matrix(releases, resolve):
    images = {tag: resolve(tag) for tag in ["latest", *release_lines(releases)]}
    return {"include": [
        {"tag": tag, "image": image, "mode": "psql"}
        for tag, image in images.items()
    ] + [{"tag": "latest", "image": images["latest"], "mode": "trustd"}]}


def resolve_image(tag):
    image = f"ghcr.io/guacsec/trustd:{tag}"
    manifest = json.loads(subprocess.check_output([
        "docker", "buildx", "imagetools", "inspect", image,
        "--format", "{{json .Manifest}}",
    ], text=True))
    digest = manifest["digest"]
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", digest):
        raise ValueError(f"invalid manifest digest for {image}")
    return f"ghcr.io/guacsec/trustd@{digest}"


def main():
    pages = json.loads(subprocess.check_output([
        "gh", "api", "--paginate", "--slurp",
        "repos/guacsec/trustify/releases?per_page=100",
    ], text=True))
    matrix = build_matrix([release for page in pages for release in page], resolve_image)
    print(json.dumps(matrix))
    if os.environ.get("GITHUB_OUTPUT"):
        with open(os.environ["GITHUB_OUTPUT"], "a") as output:
            output.write(f"matrix={json.dumps(matrix)}\n")
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as summary:
            summary.write("| Image tag | Initialization | Resolved image |\n|---|---|---|\n")
            for case in matrix["include"]:
                summary.write(f"| {case['tag']} | {case['mode']} | `{case['image']}` |\n")


if __name__ == "__main__":
    main()
