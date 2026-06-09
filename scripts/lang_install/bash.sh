#!/usr/bin/env bash
set -euo pipefail
# bash is already present in the base image (debian:trixie-slim includes bash).
# This script is a no-op smoke marker that ensures the loop runs without error.
bash --version
