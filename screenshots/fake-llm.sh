#!/usr/bin/env bash
# Fake LLM for screenshot generation.
# Returns canned responses based on prompt content.
# Reads stdin (the prompt), writes response to stdout.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
prompt="$(cat)"

if echo "$prompt" | grep -q "Return ONLY the numbers"; then
  # ask filter pass
  cat "$SCRIPT_DIR/canned-ask-filter.txt"
elif echo "$prompt" | grep -q "Answer my question"; then
  # ask answer pass
  cat "$SCRIPT_DIR/canned-ask-answer.txt"
elif echo "$prompt" | grep -q "Write a concise recap"; then
  # recap
  cat "$SCRIPT_DIR/canned-recap.txt"
else
  echo "Unknown prompt" >&2
  exit 1
fi
