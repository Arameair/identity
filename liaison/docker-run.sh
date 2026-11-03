#!/usr/bin/env bash
# Start / stop the Identity Liaison container.
#
#   ./docker-run.sh                 start model + Liaison (NVIDIA GPU)
#   ./docker-run.sh --cpu           same, without a GPU
#   ./docker-run.sh --external      Liaison only; use the model already running (TASS stack, 127.0.0.1:18081)
#   ./docker-run.sh dry-run         read waiting issues and show what it WOULD do; writes nothing
#   ./docker-run.sh check | logs | stop
set -euo pipefail
cd "$(dirname "$0")"

action=start
files=(-f compose.yaml)
external=0
for arg in "$@"; do
  case "$arg" in
    --cpu) files=(-f compose.yaml -f compose.cpu.yaml) ;;
    --external) files=(-f compose.external-model.yaml); external=1 ;;
    start|stop|logs|check|dry-run) action="$arg" ;;
    -h|--help) sed -n '2,9p' "$0"; exit 0 ;;
    *) echo "Unknown option: $arg (try --help)"; exit 2 ;;
  esac
done

export LIAISON_UID="${LIAISON_UID:-$(id -u)}" LIAISON_GID="${LIAISON_GID:-$(id -g)}"
dc() { docker compose -p identity-liaison "${files[@]}" "$@"; }
run_once() { if [[ $external == 1 ]]; then dc run --rm liaison "$@"; else dc run --rm --no-deps liaison "$@"; fi; }

case "$action" in
  stop) dc down; exit 0 ;;
  logs) dc logs -f liaison; exit 0 ;;
esac

# --- GitHub token (only the Liaison container sees it) ----------------------
mkdir -p secrets config
chmod 700 secrets
if [[ -d secrets/github_token ]]; then
  rmdir secrets/github_token 2>/dev/null || { echo "secrets/github_token is a folder; remove it."; exit 1; }
fi
if [[ ! -s secrets/github_token ]]; then
  if command -v gh >/dev/null 2>&1 && gh auth token >/dev/null 2>&1; then
    (umask 177; gh auth token > secrets/github_token)
    echo "Saved your GitHub CLI token to secrets/github_token (only the Liaison container reads it)."
  else
    echo "Need a GitHub token in secrets/github_token."
    echo "  Easiest:  gh auth login   then run this script again."
    echo "  Or:       create a token with Contents, Issues and Pull requests read/write"
    echo "            for your repos, and save it:  nano secrets/github_token"
    exit 1
  fi
fi
chmod 600 secrets/github_token

# --- model file (bundled model only) ----------------------------------------
if [[ $external == 0 ]]; then
  model_dir="${TASS_MODEL_DIR:-./models}"
  model_file="${LIAISON_MODEL_FILE:-NVIDIA-Nemotron3-Nano-4B-Q4_K_M.gguf}"
  if [[ ! -f "$model_dir/$model_file" ]]; then
    for guess in "$HOME/nemotronTass/models" "$HOME/nemotronTASS/models" "$HOME/nemotronTASS-docker/models"; do
      if [[ -f "$guess/$model_file" ]]; then model_dir="$guess"; break; fi
    done
  fi
  if [[ ! -f "$model_dir/$model_file" ]]; then
    echo "Model not found: $model_dir/$model_file"
    echo "  Point at it:  TASS_MODEL_DIR=/path/to/models ./docker-run.sh"
    echo "  Or use a model that's already running:  ./docker-run.sh --external"
    exit 1
  fi
  export TASS_MODEL_DIR="$model_dir"
  echo "Model: $TASS_MODEL_DIR/$model_file"
fi

dc build -q liaison
if [[ ! -f config/config.toml ]]; then
  run_once init >/dev/null && echo "Wrote config/config.toml (edit it to change allow-lists)."
fi

case "$action" in
  check)
    [[ $external == 0 ]] && dc up -d --wait model
    run_once check ;;
  dry-run)
    [[ $external == 0 ]] && dc up -d --wait model
    run_once once --dry-run ;;
  start)
    echo "Starting… (the first start downloads llama.cpp and loads the model; give it a minute)"
    dc up -d --wait
    run_once check || true
    echo
    echo "Liaison is running. It checks GitHub every ${LIAISON_PULSE:-300}s."
    echo "  Watch it:  ./docker-run.sh logs        Stop it:  ./docker-run.sh stop"
    echo "  Give it work: open an issue with the 'liaison' label (see README.md)." ;;
esac
