#!/bin/sh
set -eu

REPO="agentenv/agentenv"
BINARY="agentenv"
INSTALL_DIR="/usr/local/bin"

# --- helper functions ---
info()  { printf "\033[32m%s\033[0m\n" "$*"; }
warn()  { printf "\033[33m%s\033[0m\n" "$*"; }
error() { printf "\033[31m%s\033[0m\n" "$*"; exit 1; }

# --- detect OS and arch ---
detect_os_arch() {
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  ARCH="$(uname -m)"

  case "$OS" in
    darwin|linux) ;;
    *) error "Unsupported OS: $OS (only darwin/linux are supported)" ;;
  esac

  case "$ARCH" in
    x86_64|amd64) ARCH="x86_64" ;;
    aarch64|arm64) ARCH="aarch64" ;;
    *) error "Unsupported architecture: $ARCH (only amd64/arm64 are supported)" ;;
  esac

  # Normalize OS string for artifact names
  case "$OS" in
    darwin) OS_ARTIFACT="Darwin" ;;
    linux)  OS_ARTIFACT="Linux" ;;
  esac
}

# --- get latest release tag ---
fetch_latest_tag() {
  info "Fetching latest release for $REPO..."
  TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
    grep '"tag_name":' | sed 's/.*"tag_name": "//;s/".*//') 2>/dev/null || true

  if [ -z "$TAG" ]; then
    error "Failed to fetch latest release tag from GitHub.
Please check your internet connection and that the repository $REPO exists.
If you want to install a specific version, use:
  curl -fsSL https://raw.githubusercontent.com/$REPO/main/install.sh | VERSION=vX.Y.Z sh"
  fi
  info "Latest release: $TAG"
}

# --- download and verify ---
download_and_verify() {
  VERSION="${VERSION:-$TAG}"
  ARCHIVE_NAME="${BINARY}_${VERSION}_${OS_ARTIFACT}_${ARCH}.tar.gz"
  DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$ARCHIVE_NAME"
  CHECKSUMS_URL="https://github.com/$REPO/releases/download/$VERSION/checksums.txt"

  TMP_DIR=$(mktemp -d)
  trap 'rm -rf "$TMP_DIR"' EXIT

  info "Downloading $ARCHIVE_NAME..."
  curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$ARCHIVE_NAME"

  info "Downloading checksums.txt..."
  curl -fsSL "$CHECKSUMS_URL" -o "$TMP_DIR/checksums.txt"

  info "Verifying SHA256 checksum..."
  (
    cd "$TMP_DIR"
    if command -v sha256sum >/dev/null 2>&1; then
      sha256sum -c checksums.txt --ignore-missing 2>/dev/null || sha256sum -c checksums.txt 2>/dev/null || {
        error "Checksum verification failed! The downloaded file may be corrupted or tampered with."
      }
    elif command -v shasum >/dev/null 2>&1; then
      shasum -a 256 -c checksums.txt --ignore-missing 2>/dev/null || shasum -a 256 -c checksums.txt 2>/dev/null || {
        error "Checksum verification failed! The downloaded file may be corrupted or tampered with."
      }
    else
      warn "No sha256sum/shasum found — skipping checksum verification"
    fi
  )

  info "Extracting..."
  tar -xzf "$TMP_DIR/$ARCHIVE_NAME" -C "$TMP_DIR"
}

# --- install binary ---
install_binary() {
  if [ ! -w "$INSTALL_DIR" ]; then
    warn "Need root to install to $INSTALL_DIR"
    if command -v sudo >/dev/null 2>&1; then
      sudo install -m 0755 "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
    else
      error "sudo not available. Please run: sudo install -m 0755 $TMP_DIR/$BINARY $INSTALL_DIR/$BINARY"
    fi
  else
    install -m 0755 "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
  fi
  info "Installed $BINARY to $INSTALL_DIR/$BINARY"
}

# --- verify installation ---
verify_install() {
  if command -v "$BINARY" >/dev/null 2>&1; then
    info "Installation complete! Run '$BINARY --help' to get started."
  else
    warn "Installation succeeded but $BINARY is not in PATH."
    warn "Make sure $INSTALL_DIR is in your PATH, or add it:"
    warn "  export PATH=\"\$PATH:$INSTALL_DIR\""
  fi
}

# --- main ---
main() {
  detect_os_arch
  fetch_latest_tag
  download_and_verify
  install_binary
  verify_install
}

main
