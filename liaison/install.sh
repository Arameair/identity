#!/usr/bin/env bash
# Install identity-liaison on Linux / WSL.
#   curl -fsSL https://raw.githubusercontent.com/Arameair/identity/alpha/liaison/install.sh | bash
# or, from a checkout:   bash liaison/install.sh
set -euo pipefail

BRANCH="${LIAISON_BRANCH:-alpha}"
SOURCE="git+https://github.com/Arameair/identity@${BRANCH}#subdirectory=liaison"

here="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd || true)"
if [[ -n "$here" && -f "$here/pyproject.toml" ]]; then
  SOURCE="$here"   # running from a checkout: install that copy
fi

python3 - <<'PY' || { echo "Python 3.11 or newer is required."; exit 1; }
import sys; raise SystemExit(0 if sys.version_info >= (3, 11) else 1)
PY

if ! command -v pipx >/dev/null 2>&1; then
  echo "Installing pipx…"
  if command -v apt-get >/dev/null 2>&1; then
    sudo apt-get update -qq && sudo apt-get install -y -qq pipx
  else
    python3 -m pip install --user pipx --break-system-packages
  fi
  python3 -m pipx ensurepath >/dev/null 2>&1 || pipx ensurepath >/dev/null 2>&1 || true
  export PATH="$HOME/.local/bin:$PATH"
fi

echo "Installing identity-liaison from: $SOURCE"
pipx install --force "$SOURCE"

identity-liaison init
if ! command -v gh >/dev/null 2>&1; then
  echo "Note: GitHub CLI (gh) not found. Install it and run 'gh auth login', or set GITHUB_TOKEN."
fi
echo
echo "Done. Next:  identity-liaison check"
