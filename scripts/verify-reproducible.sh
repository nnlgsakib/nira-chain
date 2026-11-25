#!/usr/bin/env bash
# Build the release twice from two independent clean clones of the same commit and
# compare checksums (G-OPS-4). Identical hashes mean anyone with the same Go toolchain
# and C compiler can rebuild and verify a published nirad binary.
#   scripts/verify-reproducible.sh [git-ref]   (default HEAD)
set -euo pipefail
REF=${1:-HEAD}
REPO="$(cd "$(dirname "$0")/.." && pwd)"
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
for n in a b; do
  git clone -q "$REPO" "$WORK/$n"
  git -C "$WORK/$n" checkout -q "$REF"
  (cd "$WORK/$n" && GOFLAGS=-modcacherw make release >/dev/null)
done
A=$(cat "$WORK/a/build/release/SHA256SUMS"); B=$(cat "$WORK/b/build/release/SHA256SUMS")
echo "$A"
if [ "$A" = "$B" ]; then echo "REPRODUCIBLE: both clean builds of $(git -C "$REPO" rev-parse --short "$REF") are identical"; else echo "NOT REPRODUCIBLE"; diff <(echo "$A") <(echo "$B"); exit 1; fi
