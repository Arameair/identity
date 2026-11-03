#!/usr/bin/env bash
# Start / stop the Identity Liaison container.
#
#   ./docker-run.sh                 start model + Liaison (NVIDIA GPU)
#   ./docker-run.sh --cpu           same, without a GPU
#   ./docker-run.sh --external      Liaison only; use the model already running (TASS stack, 127.0.0.1:18081)
#   ./docker-run.sh dry-run         read waiting issues and show what it WOULD do; writes nothing
#   ./docker-run.sh check | logs | stop
#
# Run it as yourself (no sudo). If Docker needs sudo on this computer, the
# script uses sudo only for the docker commands and asks for your password.
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
    -h|--help) sed -n '2,11p' "$0"; exit 0 ;;
    *) echo "Unknown option: $arg (try --help)"; exit 2 ;;
  esac
done

# --- who is the real user? ---------------------------------------------------
# If someone typed `sudo ./docker-run.sh`, act for the person behind sudo:
# their gh login, their home folder (for the model), their file ownership.
if [[ $EUID -eq 0 && -n "${SUDO_USER:-}" && "${SUDO_USER}" != root ]]; then
  real_user="$SUDO_USER"
  real_uid="$SUDO_UID"; real_gid="$SUDO_GID"
  real_home="$(getent passwd "$SUDO_USER" | cut -d: -f6)"
  as_user() { sudo -u "$real_user" -H "$@"; }
else
  real_user="$(id -un)"; real_uid="$(id -u)"; real_gid="$(id -g)"; real_home="$HOME"
  as_user() { "$@"; }
fi
export LIAISON_UID="${LIAISON_UID:-$real_uid}" LIAISON_GID="${LIAISON_GID:-$real_gid}"

# --- can we talk to Docker? Use sudo only for docker if needed --------------
DOCKER=(docker)
if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is not installed. In WSL: install Docker Engine (docs.docker.com/engine/install/ubuntu)"
  echo "or Docker Desktop with WSL integration turned on."
  exit 1
fi
docker_err="$(docker info 2>&1 >/dev/null)" || {
  if [[ "$docker_err" == *"permission denied"* && $EUID -ne 0 ]]; then
    echo "Docker needs sudo on this computer, so docker commands will run with sudo."
    keep="LIAISON_UID,LIAISON_GID,TASS_MODEL_DIR,LIAISON_MODEL_FILE,LIAISON_MODEL_URL,LIAISON_MODE"
    keep+=",LIAISON_PULSE,LIAISON_REPOS,LIAISON_OWNERS,LIAISON_CTX,LIAISON_GPU_LAYERS,LIAISON_BASE,LIAISON_PIP_FLAGS"
    DOCKER=(sudo "--preserve-env=$keep" docker)
    sudo true || { echo "sudo was refused."; exit 1; }
    echo "(Tip: to skip sudo next time:  sudo usermod -aG docker $real_user   then close and reopen WSL.)"
  fi
  if ! docker_err="$("${DOCKER[@]}" info 2>&1 >/dev/null)"; then
    echo "Docker isn't running."
    if grep -qi microsoft /proc/version 2>/dev/null; then
      echo "  In WSL try:  sudo service docker start     (or start Docker Desktop on Windows)"
    else
      echo "  Try:  sudo systemctl start docker"
    fi
    echo "  Docker said: ${docker_err##*$'\n'}"
    exit 1
  fi
}

dc() { "${DOCKER[@]}" compose -p identity-liaison "${files[@]}" "$@"; }
run_once() { if [[ $external == 1 ]]; then dc run --rm liaison "$@"; else dc run --rm --no-deps liaison "$@"; fi; }

case "$action" in
  stop) dc down; exit 0 ;;
  logs) dc logs -f liaison; exit 0 ;;
esac

# --- folders and GitHub token (owned by you; only the Liaison container reads it)
as_user mkdir -p secrets config
if [[ -d secrets/github_token ]]; then
  # Docker creates a folder here if the file was missing when compose ran.
  rmdir secrets/github_token 2>/dev/null || { echo "secrets/github_token is a folder; remove it."; exit 1; }
fi
if [[ ! -s secrets/github_token ]]; then
  if as_user gh auth token >/dev/null 2>&1; then
    (umask 177; as_user gh auth token > secrets/github_token)
    echo "Saved $real_user's GitHub CLI token to secrets/github_token (only the Liaison container reads it)."
  else
    echo "Need a GitHub token in secrets/github_token."
    echo "  Easiest:  gh auth login   (as $real_user, without sudo) then run this script again."
    echo "  Or:       create a token with Contents, Issues and Pull requests read/write"
    echo "            for your repos, and save it:  nano secrets/github_token"
    exit 1
  fi
fi
# Make sure the files belong to you even if the script ran under sudo.
chown -R "$real_uid:$real_gid" secrets config 2>/dev/null || true
chmod 700 secrets
chmod 600 secrets/github_token

# --- model file (bundled model only) ----------------------------------------
if [[ $external == 0 ]]; then
  model_dir="${TASS_MODEL_DIR:-./models}"
  model_file="${LIAISON_MODEL_FILE:-NVIDIA-Nemotron3-Nano-4B-Q4_K_M.gguf}"
  if [[ ! -f "$model_dir/$model_file" ]]; then
    for guess in "$real_home/nemotronTass/models" "$real_home/nemotronTASS/models" \
                 "$real_home/nemotronTASS-docker/models" "$real_home/nemotronTASS-docker-liaison/models"; do
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
    if [[ $external == 0 ]]; then dc up -d --wait model; fi
    run_once check ;;
  dry-run)
    if [[ $external == 0 ]]; then dc up -d --wait model; fi
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
