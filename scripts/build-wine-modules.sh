#!/usr/bin/env bash
# Builds patched win32u.so, winex11.so and riched20.dll for the GE-Proton in versions.env.
# Run inside registry.gitlab.steamos.cloud/steamrt/steamrt4/sdk, the compiler Proton itself uses.
# Usage: build-wine-modules.sh <output dir>. Needs a few GB of disk and about 15 minutes.
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$(mkdir -p "$1" && cd "$1" && pwd)"
WORK="${WORK:-/tmp/kakaotalk-wine-build}"
# shellcheck source=versions.env
source "$ROOT/versions.env"

# riched20.dll is a PE module; the SDK image has no mingw by default.
command -v x86_64-w64-mingw32-gcc >/dev/null ||
    { apt-get update -qq && apt-get install -y -qq gcc-mingw-w64-x86-64 >/dev/null; }

git config --global user.email build@localhost
git config --global user.name build
git config --global --add safe.directory '*'

rm -rf "$WORK"
git clone -q --depth 1 --branch "$PROTON_TAG" https://github.com/GloriousEggroll/proton-ge-custom.git "$WORK"
cd "$WORK"
WINE_SHA=$(git ls-tree HEAD wine | awk '{print $3}')
STAGING_SHA=$(git ls-tree HEAD wine-staging | awk '{print $3}')
rm -rf wine wine-staging
# GE reverts upstream commits, so Wine needs its history. Blobs are fetched only when used.
git clone -q --filter=blob:none --no-checkout https://github.com/ValveSoftware/wine.git wine
git -C wine checkout -q "$WINE_SHA"
git init -q wine-staging
git -C wine-staging fetch -q --depth 1 https://github.com/wine-staging/wine-staging.git "$STAGING_SHA"
git -C wine-staging checkout -q FETCH_HEAD

# Only the "WINE PATCHING" part of GE's prepare script, plus its patch helpers.
PREP=patches/protonprep-valve-staging.sh
FIRST=$(grep -n '^### (2) WINE PATCHING ###' "$PREP" | cut -d: -f1)
LAST=$(grep -n '^### END WINE PATCHING ###' "$PREP" | cut -d: -f1)
{ sed -n '1,15p' "$PREP"; sed -n "${FIRST},${LAST}p" "$PREP"; } > /tmp/ge-wine-prep.sh
bash /tmp/ge-wine-prep.sh > "$OUT/ge-prep.log" 2>&1 || { tail -20 "$OUT/ge-prep.log"; exit 1; }
if grep -E 'FAILED|^fatal:' "$OUT/ge-prep.log"; then echo "GE patching reported failures"; exit 1; fi

cd "$WORK/wine"
patch -Np1 < "$ROOT/wine-patches/kakaotalk.patch"
[ -f configure ] || autoreconf -f
XDG_CACHE_HOME=/tmp python3 dlls/winevulkan/make_vulkan -x vk.xml -X video.xml
tools/make_requests
tools/make_specfiles || true
mkdir -p build && cd build
../configure --enable-win64 --enable-archs=x86_64 --disable-tests --without-gstreamer --without-opencl \
    --without-sane --without-cups --without-v4l2 --without-gphoto --without-pcap --without-oss \
    --without-alsa --without-pulse --without-netapi --without-sdl --without-usb --without-unwind \
    --without-krb5 --without-gssapi > "$OUT/configure.log" 2>&1
make -j"$(nproc)" dlls/win32u/win32u.so dlls/winex11.drv/winex11.so \
    dlls/riched20/x86_64-windows/riched20.dll > "$OUT/make.log" 2>&1 \
    || { grep -E 'error|\*\*\*' "$OUT/make.log" | head -20; exit 1; }
strip -o "$OUT/win32u.so" dlls/win32u/win32u.so
strip -o "$OUT/winex11.so" dlls/winex11.drv/winex11.so
x86_64-w64-mingw32-strip -o "$OUT/riched20.dll" dlls/riched20/x86_64-windows/riched20.dll
chmod 0755 "$OUT/win32u.so" "$OUT/winex11.so" "$OUT/riched20.dll"
echo modules_ok
