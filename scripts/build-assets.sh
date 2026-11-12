#!/usr/bin/env bash
# Builds release assets into dist/. Needs Go and the x86_64 mingw-w64 compiler.
set -Eeuo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
mkdir -p dist
(cd helper && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o ../dist/kakaotalk-helper .)
(cd kakao-ctl && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o ../dist/kakao-ctl.exe .)
(cd comdlg32-proxy && x86_64-w64-mingw32-gcc -O2 -Wall -municode -shared -static-libgcc -DUNICODE -D_UNICODE \
    -o ../dist/comdlg32.dll proxy.c comdlg32.def -luser32)
# win32u.so, winex11.so and riched20.dll come from build-wine-modules.sh.
(cd dist && sha256sum kakaotalk-helper kakao-ctl.exe comdlg32.dll win32u.so winex11.so riched20.dll > SHA256SUMS)
