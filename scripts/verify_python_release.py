#!/usr/bin/env python3
"""Validate Python release metadata and artifacts without publishing them."""

from __future__ import annotations

import argparse
import tarfile
import tomllib
import zipfile
from email.parser import BytesParser
from email.policy import default
from pathlib import Path

from packaging.version import Version


PROJECT_ROOT = Path(__file__).resolve().parents[1]
PACKAGE_ROOT = PROJECT_ROOT / "packages" / "python"
PACKAGE_NAME = "hostero"
TAG_PREFIX = "python-v"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--tag",
        help="Expected Git tag. Defaults to python-v followed by the project version.",
    )
    return parser.parse_args()


def load_project_version() -> str:
    with (PACKAGE_ROOT / "pyproject.toml").open("rb") as project_file:
        project = tomllib.load(project_file)
    return project["project"]["version"]


def load_lock_version() -> str:
    with (PACKAGE_ROOT / "uv.lock").open("rb") as lock_file:
        lock = tomllib.load(lock_file)

    matching_packages = [
        package
        for package in lock["package"]
        if package["name"] == PACKAGE_NAME and package.get("source", {}).get("editable") == "."
    ]
    if len(matching_packages) != 1:
        message = "uv.lock must contain exactly one editable hostero package entry"
        raise ValueError(message)
    return matching_packages[0]["version"]


def read_wheel_metadata(wheel_path: Path) -> tuple[str, bool]:
    with zipfile.ZipFile(wheel_path) as wheel:
        metadata_paths = [
            path for path in wheel.namelist() if path.endswith(".dist-info/METADATA")
        ]
        if len(metadata_paths) != 1:
            message = f"{wheel_path.name} must contain exactly one distribution METADATA file"
            raise ValueError(message)
        metadata = BytesParser(policy=default).parsebytes(wheel.read(metadata_paths[0]))
        typed = f"{PACKAGE_NAME}/py.typed" in wheel.namelist()
    return metadata["Version"], typed


def read_sdist_version(sdist_path: Path) -> tuple[str, bool]:
    with tarfile.open(sdist_path, "r:gz") as sdist:
        pyproject_members = [
            member
            for member in sdist.getmembers()
            if member.name.endswith("/pyproject.toml") and member.isfile()
        ]
        if len(pyproject_members) != 1:
            message = f"{sdist_path.name} must contain exactly one pyproject.toml"
            raise ValueError(message)
        pyproject_file = sdist.extractfile(pyproject_members[0])
        if pyproject_file is None:
            message = f"could not read {pyproject_members[0].name} from {sdist_path.name}"
            raise ValueError(message)
        project = tomllib.loads(pyproject_file.read().decode("utf-8"))
        typed = any(
            member.name.endswith(f"/src/{PACKAGE_NAME}/py.typed")
            for member in sdist.getmembers()
        )
    return project["project"]["version"], typed


def require_version(actual: str, expected: Version, artifact: Path) -> None:
    if Version(actual) != expected:
        message = f"{artifact.name} has version {actual!r}, expected {expected!s}"
        raise ValueError(message)


def main() -> None:
    args = parse_args()
    project_version = load_project_version()
    lock_version = load_lock_version()
    expected_version = Version(project_version)

    if Version(lock_version) != expected_version:
        message = f"uv.lock has hostero version {lock_version!r}, expected {project_version!r}"
        raise ValueError(message)

    expected_tag = f"{TAG_PREFIX}{project_version}"
    if args.tag and args.tag != expected_tag:
        message = f"tag {args.tag!r} does not match project version {project_version!r}"
        raise ValueError(message)

    distributions = PACKAGE_ROOT / "dist"
    wheels = sorted(distributions.glob(f"{PACKAGE_NAME}-*.whl"))
    sdists = sorted(distributions.glob(f"{PACKAGE_NAME}-*.tar.gz"))
    if len(wheels) != 1 or len(sdists) != 1:
        message = "dist must contain exactly one hostero wheel and one source distribution"
        raise ValueError(message)

    wheel_version, wheel_typed = read_wheel_metadata(wheels[0])
    sdist_version, sdist_typed = read_sdist_version(sdists[0])
    require_version(wheel_version, expected_version, wheels[0])
    require_version(sdist_version, expected_version, sdists[0])
    if not wheel_typed or not sdist_typed:
        message = "wheel and source distribution must both include hostero/py.typed"
        raise ValueError(message)

    print(f"Verified {PACKAGE_NAME} {expected_version} ({expected_tag})")


if __name__ == "__main__":
    main()
