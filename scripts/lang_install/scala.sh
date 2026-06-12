#!/usr/bin/env bash
set -euo pipefail
# default-jdk is already installed by kotlin.sh — skip to avoid redundant 400MB download
apt-get install -y --no-install-recommends scala