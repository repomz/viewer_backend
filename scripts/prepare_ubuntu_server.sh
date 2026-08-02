#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_NAME="$(basename "$0")"
readonly SCRIPT_NAME
readonly SERVER_TIMEZONE="${SERVER_TIMEZONE:-Asia/Tomsk}"
readonly INSTALL_DIR="${INSTALL_DIR:-/opt/viewer}"
readonly UPGRADE_SYSTEM="${UPGRADE_SYSTEM:-1}"
readonly ALLOW_UNSUPPORTED_UBUNTU="${ALLOW_UNSUPPORTED_UBUNTU:-0}"

log() {
  printf '\n[%s] %s\n' "$SCRIPT_NAME" "$*"
}

warn() {
  printf '\n[%s] WARNING: %s\n' "$SCRIPT_NAME" "$*" >&2
}

die() {
  printf '\n[%s] ERROR: %s\n' "$SCRIPT_NAME" "$*" >&2
  exit 1
}

on_error() {
  local exit_code=$?
  printf '\n[%s] ERROR: command failed at line %s with exit code %s\n' \
    "$SCRIPT_NAME" "${BASH_LINENO[0]}" "$exit_code" >&2
  exit "$exit_code"
}

trap on_error ERR

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  cat <<EOF
Usage:
  sudo ./${SCRIPT_NAME}

Optional environment variables:
  SERVER_TIMEZONE=Asia/Tomsk   Server timezone
  INSTALL_DIR=/opt/viewer      Directory prepared for the repository
  DOCKER_USER=username         Non-root administrator added to docker group
  UPGRADE_SYSTEM=1             Set to 0 to skip apt full-upgrade
  ALLOW_UNSUPPORTED_UBUNTU=0   Set to 1 to bypass the Ubuntu version check

The script installs Docker but does not configure networking/firewall,
clone the repository, generate application secrets, or start Compose.
EOF
  exit 0
fi

if (($# > 0)); then
  die "unknown argument '$1'; use --help"
fi

if [[ "${EUID}" -ne 0 ]]; then
  die "run this script as root: sudo ./${SCRIPT_NAME}"
fi

if [[ ! -r /etc/os-release ]]; then
  die "/etc/os-release is missing; only Ubuntu Server is supported"
fi

# shellcheck disable=SC1091
source /etc/os-release

if [[ "${ID:-}" != "ubuntu" ]]; then
  die "unsupported OS '${ID:-unknown}'; use Ubuntu Server 22.04 or 24.04"
fi

case "${VERSION_ID:-}" in
  22.04 | 24.04)
    ;;
  *)
    if [[ "$ALLOW_UNSUPPORTED_UBUNTU" != "1" ]]; then
      die "Ubuntu ${VERSION_ID:-unknown} is not validated for this project; set ALLOW_UNSUPPORTED_UBUNTU=1 to override"
    fi
    warn "continuing with unvalidated Ubuntu ${VERSION_ID:-unknown}"
    ;;
esac

ARCHITECTURE="$(dpkg --print-architecture)"
readonly ARCHITECTURE
if [[ "$ARCHITECTURE" != "amd64" ]]; then
  die "architecture '$ARCHITECTURE' is not supported by the pinned Orthanc stack; amd64 is required"
fi

readonly DOCKER_USER="${DOCKER_USER:-${SUDO_USER:-}}"
if [[ -n "$DOCKER_USER" && "$DOCKER_USER" != "root" ]]; then
  if ! id "$DOCKER_USER" >/dev/null 2>&1; then
    die "DOCKER_USER '$DOCKER_USER' does not exist"
  fi
else
  warn "no non-root administrator detected; Docker access and ${INSTALL_DIR} will remain root-only"
fi

export DEBIAN_FRONTEND=noninteractive
export NEEDRESTART_MODE=a

log "Updating Ubuntu package metadata"
apt-get update

if [[ "$UPGRADE_SYSTEM" == "1" ]]; then
  log "Installing available Ubuntu updates"
  apt-get full-upgrade -y
else
  warn "system upgrade skipped because UPGRADE_SYSTEM=${UPGRADE_SYSTEM}"
fi

log "Installing base administration tools"
apt-get install -y \
  ca-certificates \
  curl \
  git \
  jq \
  netcat-openbsd \
  openssl

conflicting_packages=(
  docker.io
  docker-compose
  docker-compose-v2
  docker-doc
  podman-docker
  containerd
  runc
)
installed_conflicts=()

for package in "${conflicting_packages[@]}"; do
  if dpkg-query -W -f='${db:Status-Abbrev}' "$package" 2>/dev/null | grep -q '^ii'; then
    installed_conflicts+=("$package")
  fi
done

if ((${#installed_conflicts[@]} > 0)); then
  log "Removing conflicting packages: ${installed_conflicts[*]}"
  apt-get remove -y "${installed_conflicts[@]}"
else
  log "No conflicting Docker packages found"
fi

log "Adding Docker's official APT signing key"
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
  -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc

readonly UBUNTU_SUITE="${UBUNTU_CODENAME:-${VERSION_CODENAME:-}}"
if [[ -z "$UBUNTU_SUITE" ]]; then
  die "cannot determine Ubuntu codename"
fi

log "Adding Docker's official APT repository for ${UBUNTU_SUITE}/${ARCHITECTURE}"
cat >/etc/apt/sources.list.d/docker.sources <<EOF
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: ${UBUNTU_SUITE}
Components: stable
Architectures: ${ARCHITECTURE}
Signed-By: /etc/apt/keyrings/docker.asc
EOF

apt-get update

log "Installing Docker Engine, Buildx and Docker Compose v2"
apt-get install -y \
  docker-ce \
  docker-ce-cli \
  containerd.io \
  docker-buildx-plugin \
  docker-compose-plugin

log "Configuring bounded Docker container logs"
install -m 0755 -d /etc/docker
if [[ ! -e /etc/docker/daemon.json ]]; then
  cat >/etc/docker/daemon.json <<'EOF'
{
  "log-driver": "local",
  "log-opts": {
    "max-size": "20m",
    "max-file": "5"
  }
}
EOF
else
  warn "/etc/docker/daemon.json already exists; log settings were not overwritten"
  jq empty /etc/docker/daemon.json \
    || die "/etc/docker/daemon.json contains invalid JSON"
fi

log "Enabling and starting Docker"
systemctl daemon-reload
systemctl enable docker
systemctl restart docker
systemctl is-active --quiet docker \
  || die "Docker service is not active"

log "Configuring server timezone"
timedatectl set-timezone "$SERVER_TIMEZONE"
if ! timedatectl set-ntp true; then
  warn "could not enable NTP automatically; verify time synchronization manually"
fi

if [[ -n "$DOCKER_USER" && "$DOCKER_USER" != "root" ]]; then
  log "Adding '${DOCKER_USER}' to the docker group"
  usermod -aG docker "$DOCKER_USER"
  install -d -m 0755 -o "$DOCKER_USER" -g "$DOCKER_USER" "$INSTALL_DIR"
else
  install -d -m 0755 "$INSTALL_DIR"
fi

log "Verifying Docker installation"
docker version
docker compose version
docker run --rm hello-world >/dev/null

printf '\nServer preparation completed successfully.\n'
printf 'Docker Engine:  %s\n' "$(docker version --format '{{.Server.Version}}')"
printf 'Docker Compose: %s\n' "$(docker compose version --short)"
printf 'Project folder: %s\n' "$INSTALL_DIR"

if [[ -n "$DOCKER_USER" && "$DOCKER_USER" != "root" ]]; then
  printf '\nLog out and reconnect over SSH so docker-group membership takes effect.\n'
fi

if [[ -f /var/run/reboot-required ]]; then
  printf '\nA system reboot is required before deployment:\n'
  printf '  sudo reboot\n'
fi

cat <<EOF

Actions intentionally NOT performed by this script:
  - static IP and DNS configuration;
  - firewall or cloud Security Group changes;
  - repository cloning;
  - application secret generation;
  - Docker Compose application startup.

After reconnecting (and rebooting if requested), continue with:
  cd ${INSTALL_DIR}
  git clone https://github.com/repomz/viewer_backend.git
  cd viewer_backend
  cp .env.compose.example .env

Then follow docker_install.md from the repository configuration section.
EOF
