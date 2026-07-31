#!/usr/bin/env python3
"""Install optional repository agent support files for knowledge-compiler."""

from __future__ import annotations

import argparse
import shutil
from pathlib import Path


def copy_tree(source: Path, target: Path, overwrite: bool) -> list[str]:
    copied = []
    for path in sorted(source.rglob("*")):
        if path.is_dir():
            continue
        relative = path.relative_to(source)
        destination = target / relative
        if destination.exists() and not overwrite:
            continue
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(path, destination)
        copied.append(destination.as_posix())
    return copied


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--project", default=".", help="Project root")
    parser.add_argument("--overwrite", action="store_true", help="Replace existing support files")
    args = parser.parse_args()

    skill_dir = Path(__file__).resolve().parents[1]
    source = skill_dir / "assets" / "repository-support"
    if not source.exists():
        raise SystemExit(f"Missing repository support assets: {source}")

    project = Path(args.project).resolve()
    copied = copy_tree(source, project, args.overwrite)
    if copied:
        print("installed repository support files:")
        for item in copied:
            print(f"- {item}")
    else:
        print("no files installed; existing files were preserved")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
