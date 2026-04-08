#!/usr/bin/env bash
# Run sesh with fake screenshot data.
#
# Usage:
#   ./screenshots/run.sh              # open the picker
#   ./screenshots/run.sh list         # non-interactive list
#   ./screenshots/run.sh stats        # session statistics
#   ./screenshots/run.sh show ses_a8  # show a session
#
# Regenerate data first (timestamps are relative to "now"):
#   ./screenshots/generate-data.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Regenerate timestamps so relative times are fresh.
bash "$SCRIPT_DIR/generate-data.sh" >/dev/null

# Point sesh at our screenshot config.
# XDG_CONFIG_HOME is checked first in loadConfig(), so this takes priority
# over ~/.config/sesh/config.json.
export XDG_CONFIG_HOME="$SCRIPT_DIR"

# The list_command uses relative paths ("cat opencode.json"), so run from
# the screenshots directory.
cd "$SCRIPT_DIR"

# Use the locally-built binary if available, otherwise fall back to PATH.
SESH="$REPO_DIR/sesh"
if [[ ! -x "$SESH" ]]; then
  SESH="$(command -v sesh)"
fi

exec "$SESH" "$@"
