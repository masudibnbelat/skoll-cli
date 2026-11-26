#!/usr/bin/env bash
# skoll installer for Linux (and macOS).
# Usage — paste this one line in a terminal:
#   curl -fsSL https://raw.githubusercontent.com/masudibnbelat/skoll-cli/main/install.sh | bash
#
# What it does:
#   1. Finds the latest GitHub Release of masudibnbelat/skoll-cli.
#   2. Downloads the matching binary (linux-amd64 or linux-arm64).
#   3. Installs it to ~/.local/bin/skoll and makes it executable.
#   4. Makes sure ~/.local/bin is on PATH (adds it to ~/.bashrc / ~/.zshrc).
set -e
REPO="masudibnbelat/skoll-cli"
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
  linux) ASSET="skoll-linux-$ARCH" ;;
  darwin) ASSET="skoll-darwin-$ARCH" ;;
  *) echo "STOP: this script is for Linux/macOS. On Windows use install.ps1 in PowerShell."; exit 1 ;;
esac

echo "== skoll: finding latest release of $REPO"
# The GitHub web redirect gives the latest tag with no API call, so
# it never hits the unauthenticated API rate limit (60/hour per IP —
# that is what a 403 from api.github.com means). Fall back to the
# API only if the redirect fails; GITHUB_TOKEN is honoured when set.
TAG="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" | sed -E 's#.*/tag/##')"
if [ -z "$TAG" ]; then
  if [ -n "$GITHUB_TOKEN" ]; then
    AUTH=(-H "Authorization: Bearer $GITHUB_TOKEN")
  else
    AUTH=()
  fi
  TAG="$(curl -fsSL "${AUTH[@]}" "https://api.github.com/repos/$REPO/releases/latest" | grep -m1 '"tag_name":' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
fi
[ -n "$TAG" ] || { echo "STOP: could not find the latest release. If GitHub answered 403, the API rate limit (60/hour per IP) is hit — wait an hour, or set GITHUB_TOKEN and retry. If no release exists yet, publish one: git tag v0.6.0 && git push origin v0.6.0"; exit 1; }
echo "== latest: $TAG — downloading $ASSET"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
curl -fsSL -o "$TMP/skoll" "https://github.com/$REPO/releases/download/$TAG/$ASSET"
chmod +x "$TMP/skoll"
cp "$TMP/skoll" "$BIN_DIR/skoll"

# Make sure ~/.local/bin is on PATH for future shells.
for rc in "$HOME/.bashrc" "$HOME/.zshrc"; do
  if [ -f "$rc" ] && ! grep -q '\.local/bin' "$rc"; then
    printf '\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "$rc"
  fi
done
export PATH="$BIN_DIR:$PATH"

echo "== DONE: skoll $TAG installed to $BIN_DIR/skoll"
skoll --version
echo "Try it in any Android project:"
echo "  cd <your-project>   # e.g. MyApp"
echo "  skoll run"
echo "  skoll release v1.5.0 \"notes\""
