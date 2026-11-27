#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
dotnet build Fixture.csproj --configuration Debug --artifacts-path "$scratch" --output ../../bin/linux-amd64/csharp
