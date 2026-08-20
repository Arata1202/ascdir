#!/bin/sh
set -eu

repository="Arata1202/ascdir"
binary="ascdir"

if [ -n "${INSTALL_DIR:-}" ]; then
  install_dir="$INSTALL_DIR"
elif [ -w /usr/local/bin ]; then
  install_dir="/usr/local/bin"
else
  install_dir="${HOME}/.local/bin"
fi
mkdir -p "$install_dir"

case "$(uname -s)" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *) echo "unsupported operating system: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

version="${ASCDIR_VERSION:-}"
if [ -z "$version" ]; then
  version="$(curl -fsSL "https://api.github.com/repos/${repository}/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)"
fi
if [ -z "$version" ]; then
  echo "could not determine the latest ascdir release" >&2
  exit 1
fi

archive="${binary}_${version#v}_${os}_${arch}.tar.gz"
base_url="https://github.com/${repository}/releases/download/${version}"
temporary_dir="$(mktemp -d)"
trap 'rm -rf "$temporary_dir"' EXIT HUP INT TERM

curl -fsSL "${base_url}/${archive}" -o "${temporary_dir}/${archive}"
curl -fsSL "${base_url}/checksums.txt" -o "${temporary_dir}/checksums.txt"
expected="$(awk -v archive="$archive" '$2 == archive { print $1 }' "${temporary_dir}/checksums.txt")"
if [ -z "$expected" ]; then
  echo "checksum for ${archive} was not found" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "${temporary_dir}/${archive}" | awk '{ print $1 }')"
else
  actual="$(shasum -a 256 "${temporary_dir}/${archive}" | awk '{ print $1 }')"
fi
if [ "$actual" != "$expected" ]; then
  echo "checksum verification failed for ${archive}" >&2
  exit 1
fi

entries_file="${temporary_dir}/archive-entries.txt"
tar -tzf "${temporary_dir}/${archive}" > "$entries_file"
binary_count=0
while IFS= read -r entry; do
  case "$entry" in
    "$binary"|"./$binary") binary_count=$((binary_count + 1)) ;;
    "CHANGELOG.md"|"./CHANGELOG.md"|"LICENSE"|"./LICENSE"|"README.md"|"./README.md") ;;
    *)
      echo "archive contains an unexpected path: ${entry}" >&2
      exit 1
      ;;
  esac
done < "$entries_file"
if [ "$binary_count" -ne 1 ]; then
  echo "archive must contain exactly one ${binary} executable" >&2
  exit 1
fi
entry_type="$(tar -tvzf "${temporary_dir}/${archive}" | awk -v binary="$binary" '$NF == binary || $NF == "./" binary { print substr($1, 1, 1); exit }')"
if [ "$entry_type" != "-" ]; then
  echo "archive entry for ${binary} must be a regular file" >&2
  exit 1
fi

tar -xzf "${temporary_dir}/${archive}" -C "$temporary_dir"
if [ ! -f "${temporary_dir}/${binary}" ] || [ -L "${temporary_dir}/${binary}" ]; then
  echo "archive did not produce a regular ${binary} executable" >&2
  exit 1
fi
install -m 0755 "${temporary_dir}/${binary}" "${install_dir}/${binary}"
echo "Installed ${binary} ${version} to ${install_dir}/${binary}"
case ":${PATH}:" in
  *":${install_dir}:"*) ;;
  *) echo "Add ${install_dir} to PATH before running ascdir." ;;
esac
"${install_dir}/${binary}" --version
