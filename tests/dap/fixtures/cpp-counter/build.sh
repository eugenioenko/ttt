#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
c++ -g -O0 "-fdebug-prefix-map=$PWD=." -o ../../bin/linux-amd64/cpp-counter main.cpp
