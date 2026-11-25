#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
cc -g -O0 "-fdebug-prefix-map=$PWD=." -o ../../bin/linux-amd64/c-counter main.c
