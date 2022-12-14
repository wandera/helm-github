#!/usr/bin/env bash
set -euo pipefail

if [ -n "${HELM_GITHUB_PLUGIN_NO_INSTALL_HOOK:-}" ]; then
  echo "Development mode: not downloading versioned release."
  exit 0
fi

validate_checksum() {
  if ! grep -q "${1}" "${2}"; then
    echo "Invalid checksum" >/dev/stderr
    exit 1
  fi
  echo "Checksum is valid."
}

on_exit() {
  exit_code=$?
  if [ ${exit_code} -ne 0 ]; then
    echo "helm-github install hook failed. Please remove the plugin using 'helm plugin remove github' and install again." >/dev/stderr
  fi
  exit ${exit_code}
}
trap on_exit EXIT

version="$(cat plugin.yaml | grep "version" | cut -d '"' -f 2)"
echo "Downloading and installing helm-github v${version} ..."

arch=""
case "$(uname -m)" in
x86_64 | amd64) arch="amd64" ;;
aarch64 | arm64) arch="arm64" ;;
*)
  echo "Arch '$(uname -m)' not supported!" >&2
  exit 1
  ;;
esac

os=""
case "$(uname)" in
Darwin) os="darwin" ;;
Linux) os="linux" ;;
CYGWIN* | MINGW* | MSYS_NT*) os="windows" ;;
*)
  echo "OS '$(uname)' not supported!" >&2
  exit 1
  ;;
esac

binary_url="https://github.com/wandera/helm-github/releases/download/v${version}/helm-github_${version}_${os}_${arch}.tar.gz"
checksum_url="https://github.com/wandera/helm-github/releases/download/v${version}/helm-github_${version}_${os}_${arch}_checksum.txt"

mkdir -p "bin"
mkdir -p "releases/v${version}"
binary_filename="releases/v${version}.tar.gz"
checksum_filename="releases/v${version}_checksum.txt"

# Download binary and checksums files.
(
  if command -v curl >/dev/null 2>&1; then
    curl -sSL "${binary_url}" -o "${binary_filename}"
    curl -sSL "${checksum_url}" -o "${checksum_filename}"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "${binary_url}" -O "${binary_filename}"
    wget -q "${checksum_url}" -O "${checksum_filename}"
  else
    echo "ERROR: no curl or wget found to download files." >/dev/stderr
  fi
)

# Verify checksum.
(
  if command -v sha256sum >/dev/null 2>&1; then
    checksum=$(sha256sum "${binary_filename}" | awk '{ print $1 }')
    validate_checksum "${checksum}" "${checksum_filename}"
  elif command -v openssl >/dev/null 2>&1; then
    checksum=$(openssl dgst -sha256 "${binary_filename}" | awk '{ print $2 }')
    validate_checksum "${checksum}" "${checksum_filename}"
  else
    echo "WARNING: no tool found to verify checksum" >/dev/stderr
  fi
)

# Unpack the binary.
tar xzf "${binary_filename}" -C "releases/v${version}"
mv "releases/v${version}/bin/helmgithub" "bin/helmgithub"
exit 0
