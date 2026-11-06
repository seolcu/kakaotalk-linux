#!/usr/bin/env bash
# Installs KakaoTalk for this user. Needs no root, and changes nothing outside your home folder.
set -Eeuo pipefail

REPO=seolcu/kakaotalk-linux
RELEASE=v0.2.0
SETUP_URL=https://app-pc.kakaocdn.net/talk/win32/x64/KakaoTalk_Setup.exe
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE="${XDG_DATA_HOME:-$HOME/.local/share}/kakaotalk-linux"
BIN="$HOME/.local/bin"
APPS="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
ICONS="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor/256x256/apps"
CACHE="${XDG_CACHE_HOME:-$HOME/.cache}/kakaotalk-linux"

say() { printf '\n==> %s\n' "$1"; }
die() { printf '\n오류: %s\n' "$1" >&2; exit 1; }

# shellcheck source=versions.env
source "$SRC/versions.env"

fetch() { # url file sha256 (empty to skip the check)
    if [ ! -f "$2" ] || { [ -n "$3" ] && ! echo "$3  $2" | sha256sum -c --status -; }; then
        curl -fL --retry 3 --progress-bar -o "$2.part" "$1"
        mv "$2.part" "$2"
    fi
    if [ -n "$3" ]; then
        echo "$3  $2" | sha256sum -c --status - || die "$(basename "$2") 체크섬이 맞지 않아요. 다시 실행해 주세요."
    fi
}

say "설치 조건 확인"
[ "$(uname -m)" = x86_64 ] || die "x86_64 PC만 지원해요."
[ "$(id -u)" != 0 ] || die "root가 아닌 일반 사용자로 실행해 주세요."
for tool in curl tar sha256sum python3 install; do
    command -v "$tool" >/dev/null || die "$tool 이(가) 필요해요. 패키지 관리자로 설치해 주세요."
done
fc-list :lang=ko 2>/dev/null | grep -q . || printf '주의: 한글 글꼴이 없어요. Noto Sans CJK 글꼴 설치를 권해요.\n'

mkdir -p "$BASE" "$BIN" "$APPS" "$ICONS" "$CACHE"

say "umu-launcher $UMU_VERSION 받기"
fetch "https://github.com/Open-Wine-Components/umu-launcher/releases/download/$UMU_VERSION/umu-launcher-$UMU_VERSION-zipapp.tar" \
    "$CACHE/umu-$UMU_VERSION.tar" "$UMU_SHA256"
tar xf "$CACHE/umu-$UMU_VERSION.tar" -C "$BASE"

say "$PROTON_TAG 받기 (약 570MB)"
if [ ! -f "$BASE/$PROTON_DIR/proton" ]; then
    fetch "https://github.com/GloriousEggroll/proton-ge-custom/releases/download/$PROTON_TAG/$PROTON_TAG-x86_64.tar.gz" \
        "$CACHE/$PROTON_TAG.tar.gz" "$PROTON_SHA256"
    tar xzf "$CACHE/$PROTON_TAG.tar.gz" -C "$BASE"
fi

say "kakaotalk-linux 구성 요소 설치"
if [ -n "${KAKAOTALK_LINUX_ASSETS:-}" ]; then
    ASSETS="$KAKAOTALK_LINUX_ASSETS"
else
    ASSETS="$CACHE/assets-$RELEASE"
    mkdir -p "$ASSETS"
    for name in kakaotalk-helper kakao-ctl.exe comdlg32.dll win32u.so winex11.so SHA256SUMS; do
        fetch "https://github.com/$REPO/releases/download/$RELEASE/$name" "$ASSETS/$name" ""
    done
    (cd "$ASSETS" && sha256sum -c --status SHA256SUMS) || die "구성 요소 체크섬이 맞지 않아요."
fi
install -m 0755 "$ASSETS/kakaotalk-helper" "$BASE/kakaotalk-helper"
install -m 0644 "$ASSETS/kakao-ctl.exe" "$ASSETS/comdlg32.dll" "$BASE/"
install -m 0644 "$SRC/fonts/NotoEmoji-Regular.ttf" "$SRC/versions.env" "$BASE/"
# Patched Wine modules, built for exactly this Proton version: see wine-patches/.
install -m 0755 "$ASSETS/win32u.so" "$ASSETS/winex11.so" "$BASE/$PROTON_DIR/files/lib/wine/x86_64-unix/"
install -m 0755 "$SRC/kakaotalk" "$BIN/kakaotalk"

APP="$BASE/prefix/drive_c/Program Files/Kakao/KakaoTalk/KakaoTalk.exe"
if [ ! -f "$APP" ]; then
    say "카카오 공식 설치 파일 받기"
    fetch "$SETUP_URL" "$CACHE/KakaoTalk_Setup.exe" ""
    say "카카오톡 설치 (처음에는 실행 환경도 받아서 몇 분 걸려요)"
    # winemenubuilder would add Wine's own menu entries and desktop shortcuts.
    WINEDLLOVERRIDES="winemenubuilder.exe=d" WINEPREFIX="$BASE/prefix" GAMEID=umu-kakaotalk STORE=none \
        PROTONPATH="$BASE/$PROTON_DIR" PROTON_USE_XALIA=0 \
        python3 -I "$BASE/umu/umu-run" "$CACHE/KakaoTalk_Setup.exe" /S
    [ -f "$APP" ] || die "카카오톡 설치에 실패했어요."
fi

"$BASE/kakaotalk-helper" extract-icon "$APP" "$ICONS/kakaotalk.png" || printf '주의: 아이콘을 만들지 못했어요.\n'
sed "s|@BIN@|$BIN|" "$SRC/kakaotalk.desktop" > "$APPS/kakaotalk.desktop"
chmod 0644 "$APPS/kakaotalk.desktop"
command -v update-desktop-database >/dev/null && update-desktop-database "$APPS" 2>/dev/null || true
command -v gtk-update-icon-cache >/dev/null && gtk-update-icon-cache -q "${ICONS%/256x256/apps}" 2>/dev/null || true

say "설치 완료. 앱 메뉴에서 '카카오톡'을 실행하세요."
case "${XDG_CURRENT_DESKTOP:-}" in
    *GNOME*)
        if ! gdbus call --session --dest org.freedesktop.DBus --object-path /org/freedesktop/DBus \
            --method org.freedesktop.DBus.NameHasOwner org.kde.StatusNotifierWatcher 2>/dev/null | grep -q true; then
            printf '\nGNOME에서 트레이 아이콘을 보려면 "AppIndicator and KStatusNotifierItem Support" 확장이 필요해요.\n'
        fi
        ;;
esac
