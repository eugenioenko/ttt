#!/usr/bin/env bash
set -euo pipefail

: "${GITHUB_OUTPUT:?GITHUB_OUTPUT must name the workflow output file}"
placeholder='sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA='
scratch=$(mktemp -d)
cp flake.nix "$scratch/original"
keep=false
cleanup() {
  if [[ "$keep" == false ]]; then
    cp "$scratch/original" flake.nix
  fi
  rm -rf "$scratch"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

hash_pattern='vendorHash[[:space:]]*=[[:space:]]*"sha256-[A-Za-z0-9+/]{43}="[[:space:]]*;'
if [[ $(grep -Eo "$hash_pattern" "$scratch/original" | wc -l) -ne 1 ]]; then
  echo 'Expected exactly one SHA-256 vendorHash assignment in flake.nix' >&2
  exit 1
fi
original_hash=$(grep -Eo "$hash_pattern" "$scratch/original" | cut -d '"' -f 2)
sed -E "s|(vendorHash[[:space:]]*=[[:space:]]*\")[^\"]*(\")|\1$placeholder\2|" "$scratch/original" > flake.nix

probe_status=0
nix build .#default --no-link > "$scratch/probe" 2>&1 || probe_status=$?
cat "$scratch/probe" >&2
if [[ "$probe_status" -eq 0 ]]; then
  echo 'Placeholder vendorHash build unexpectedly succeeded' >&2
  exit 1
fi

if ! candidate=$(awk -v placeholder="$placeholder" '
  /hash mismatch/ {
    mismatches++
    if ($0 !~ "^[[:space:]]*error: hash mismatch in fixed-output derivation \047/nix/store/[^/]+-ttt-[^/]+-go-modules[.]drv\047:[[:space:]]*$") bad = 1
    stage = 1
    next
  }
  /^[[:space:]]*specified:/ {
    specified++
    if (stage != 1 || NF != 2 || $2 != placeholder) bad = 1
    stage = 2
    next
  }
  /^[[:space:]]*got:/ {
    got++
    if (stage != 2 || NF != 2) bad = 1
    candidate = $2
    stage = 3
    next
  }
  NF && (stage == 1 || stage == 2) { bad = 1 }
  END {
    if (bad || mismatches != 1 || specified != 1 || got != 1) exit 1
    print candidate
  }
' "$scratch/probe"); then
  echo "Expected one vendor derivation mismatch for the placeholder (nix exited $probe_status)" >&2
  exit 1
fi
if [[ ! "$candidate" =~ ^sha256-[A-Za-z0-9+/]{43}=$ || "$candidate" == "$placeholder" ]]; then
  echo "Invalid replacement vendorHash: $candidate" >&2
  exit 1
fi

if [[ "$candidate" == "$original_hash" ]]; then
  cp "$scratch/original" flake.nix
  echo 'changed=false' >> "$GITHUB_OUTPUT"
else
  sed -E "s|(vendorHash[[:space:]]*=[[:space:]]*\")[^\"]*(\")|\1$candidate\2|" "$scratch/original" > flake.nix
  verification_status=0
  nix build .#default --no-link > "$scratch/verification" 2>&1 || verification_status=$?
  cat "$scratch/verification" >&2
  if [[ "$verification_status" -ne 0 ]]; then
    echo "Replacement vendorHash verification failed (nix exited $verification_status)" >&2
    exit 1
  fi
  echo 'changed=true' >> "$GITHUB_OUTPUT"
fi
keep=true
