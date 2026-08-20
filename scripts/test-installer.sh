#!/bin/sh
set -eu

project_dir="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
test_dir="$(mktemp -d)"
trap 'rm -rf "$test_dir"' EXIT HUP INT TERM

fixture_dir="${test_dir}/fixtures"
fake_bin="${test_dir}/bin"
install_dir="${test_dir}/install"
mkdir -p "$fixture_dir" "$fake_bin" "$install_dir"

cat > "${fake_bin}/curl" <<'EOF'
#!/bin/sh
set -eu

output=""
url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o)
      output="$2"
      shift 2
      ;;
    -*) shift ;;
    *)
      url="$1"
      shift
      ;;
  esac
done

test -n "$output"
file="${url##*/}"
cp "${FIXTURE_DIR}/${file}" "$output"
EOF
chmod +x "${fake_bin}/curl"

write_checksum() {
  archive="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    digest="$(sha256sum "${fixture_dir}/${archive}" | awk '{ print $1 }')"
  else
    digest="$(shasum -a 256 "${fixture_dir}/${archive}" | awk '{ print $1 }')"
  fi
  printf '%s  %s\n' "$digest" "$archive" > "${fixture_dir}/checksums.txt"
}

version="v9.9.9"
case "$(uname -s)" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *) echo "unsupported test operating system" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "unsupported test architecture" >&2; exit 1 ;;
esac
archive="ascdir_9.9.9_${os}_${arch}.tar.gz"
payload_dir="${test_dir}/payload"
mkdir -p "$payload_dir"
cat > "${payload_dir}/ascdir" <<'EOF'
#!/bin/sh
echo "ascdir v9.9.9"
EOF
chmod +x "${payload_dir}/ascdir"
printf '%s\n' changelog > "${payload_dir}/CHANGELOG.md"
printf '%s\n' license > "${payload_dir}/LICENSE"
printf '%s\n' readme > "${payload_dir}/README.md"
tar -czf "${fixture_dir}/${archive}" -C "$payload_dir" CHANGELOG.md LICENSE README.md ascdir
write_checksum "$archive"

PATH="${fake_bin}:${PATH}" \
  FIXTURE_DIR="$fixture_dir" \
  INSTALL_DIR="$install_dir" \
  ASCDIR_VERSION="$version" \
  sh "${project_dir}/install.sh" > "${test_dir}/install.log"
test -x "${install_dir}/ascdir"
test "$("${install_dir}/ascdir" --version)" = "ascdir v9.9.9"

rm -f "${install_dir}/ascdir"
mkdir -p "${payload_dir}/nested"
cp "${payload_dir}/ascdir" "${payload_dir}/nested/ascdir"
tar -czf "${fixture_dir}/${archive}" -C "$payload_dir" nested/ascdir
write_checksum "$archive"
if PATH="${fake_bin}:${PATH}" \
  FIXTURE_DIR="$fixture_dir" \
  INSTALL_DIR="$install_dir" \
  ASCDIR_VERSION="$version" \
  sh "${project_dir}/install.sh" > "${test_dir}/unsafe.log" 2>&1; then
  echo "installer accepted an archive with an unexpected path" >&2
  exit 1
fi
test ! -e "${install_dir}/ascdir"

rm -rf "$payload_dir"
mkdir -p "$payload_dir"
ln -s /etc/passwd "${payload_dir}/ascdir"
tar -czf "${fixture_dir}/${archive}" -C "$payload_dir" ascdir
write_checksum "$archive"
if PATH="${fake_bin}:${PATH}" \
  FIXTURE_DIR="$fixture_dir" \
  INSTALL_DIR="$install_dir" \
  ASCDIR_VERSION="$version" \
  sh "${project_dir}/install.sh" > "${test_dir}/symlink.log" 2>&1; then
  echo "installer accepted a symlink payload" >&2
  exit 1
fi
test ! -e "${install_dir}/ascdir"

echo "installer smoke tests passed"
