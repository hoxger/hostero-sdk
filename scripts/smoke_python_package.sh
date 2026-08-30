#!/usr/bin/env sh

set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <distribution-directory>" >&2
  exit 2
fi

distribution_directory=$1

for artifact in "$distribution_directory"/*.whl "$distribution_directory"/*.tar.gz; do
  if [ ! -f "$artifact" ]; then
    echo "no artifacts found in $distribution_directory" >&2
    exit 1
  fi

  temporary_directory=$(mktemp -d)
  cleanup() {
    rm -rf "$temporary_directory"
  }
  trap cleanup EXIT HUP INT TERM

  uv venv --python 3.11 "$temporary_directory/venv"
  uv pip install --python "$temporary_directory/venv/bin/python" "$artifact"
  "$temporary_directory/venv/bin/python" -c '
from importlib.metadata import version
from importlib.resources import files

from hostero import Hostero

assert Hostero.__name__ == "Hostero"
assert (files("hostero") / "py.typed").is_file()
print("smoke-tested hostero {}".format(version("hostero")))
'

  trap - EXIT HUP INT TERM
  cleanup
done
