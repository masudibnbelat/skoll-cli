#!/usr/bin/env bash
# angrboda installer for Linux (and macOS).
# Usage — paste this one line in a terminal:
#   curl -fsSL https://raw.githubusercontent.com/masudibnbelat/angrboda-cli/main/install.sh | bash
#
# What it does:
#   1. Finds the latest GitHub Release of masudibnbelat/angrboda-cli.
#   2. Downloads the matching binary (linux-amd64 or linux-arm64).
#   3. Installs it to ~/.local/bin/angrboda and makes it executable.
#   4. Makes sure ~/.local/bin is on PATH (adds it to ~/.bashrc / ~/.zshrc).
set -e
REPO="masudibnbelat/angrboda-cli"
BIN_DIR="$HOME/.local/bin"
mkdir -p "$BIN_DIR"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "STOP: unsupported architecture: $ARCH"; exit 1 ;;
esac
case "$OS" in
  linux) ASSET="angrboda-linux-$ARCH" ;;
  darwin) ASSET="angrboda-darwin-$ARCH" ;;
  *) echo "STOP: this script is for Linux/macOS. On Windows use install.ps1 in PowerShell."; exit 1 ;;
esac

echo "== angrboda: finding latest release of $REPO"
TAG="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep -m1 '"tag_name":' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
[ -n "$TAG" ] || { echo "STOP: could not find the latest release. Has a release (tag vX.Y.Z) been published yet?"; exit 1; }
echo "== latest: $TAG — downloading $ASSET"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
curl -fsSL -o "$TMP/angrboda" "https://github.com/$REPO/releases/download/$TAG/$ASSET"
chmod +x "$TMP/angrboda"
cp "$TMP/angrboda" "$BIN_DIR/angrboda"

# Make sure ~/.local/bin is on PATH for future shells.
for rc in "$HOME/.bashrc" "$HOME/.zshrc"; do
  if [ -f "$rc" ] && ! grep -q '\.local/bin' "$rc"; then
    printf '\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "$rc"
  fi
done
export PATH="$BIN_DIR:$PATH"

echo "== DONE: angrboda $TAG installed to $BIN_DIR/angrboda"
angrboda --version
echo "Try it in any Android project:"
echo "  cd <your-project>   # e.g. MyApp"
echo "  angrboda run"
echo "  angrboda release v1.5.0 \"notes\""
