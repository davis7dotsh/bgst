#!/bin/sh
set -eu

repo="davis7dotsh/bgst"
base_url="${BGST_BASE_URL:-https://github.com/$repo/releases/latest/download}"
install_dir="${BGST_INSTALL_DIR:-$HOME/.local/bin}"

case "$(uname -s)" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *) echo "bgst: unsupported operating system: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "bgst: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

asset="bgst-$os-$arch"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT INT TERM

download() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1" -o "$2"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$2" "$1"
  else
    echo "bgst: curl or wget is required" >&2
    exit 1
  fi
}

echo "Downloading bgst for $os/$arch…"
download "$base_url/$asset" "$tmp_dir/$asset"
download "$base_url/checksums.txt" "$tmp_dir/checksums.txt"

expected="$(awk -v name="$asset" '$2 == name || $2 == "*" name { print $1 }' "$tmp_dir/checksums.txt")"
if [ -z "$expected" ]; then
  echo "bgst: release checksum is missing for $asset" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp_dir/$asset" | awk '{print $1}')"
else
  actual="$(shasum -a 256 "$tmp_dir/$asset" | awk '{print $1}')"
fi
if [ "$actual" != "$expected" ]; then
  echo "bgst: checksum verification failed" >&2
  exit 1
fi

mkdir -p "$install_dir"
install -m 0755 "$tmp_dir/$asset" "$install_dir/bgst"
echo "Installed bgst to $install_dir/bgst"

case ":${PATH:-}:" in
  *":$install_dir:"*) ;;
  *) echo "Add $install_dir to PATH to run bgst from anywhere." ;;
esac
