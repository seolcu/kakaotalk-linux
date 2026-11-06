#!/usr/bin/env bash
# Removes KakaoTalk and everything install.sh added, including local chat data.
set -Eeuo pipefail

DATA="${XDG_DATA_HOME:-$HOME/.local/share}"
BASE="$DATA/kakaotalk-linux"

read -r -p "카카오톡과 이 PC의 대화 데이터를 지울까요? 서버에 있는 대화는 지워지지 않아요. [y/N] " answer
[ "$answer" = y ] || [ "$answer" = Y ] || exit 0

if [ -x "$BASE/kakaotalk-helper" ] && "$BASE/kakaotalk-helper" running; then
    echo "카카오톡을 먼저 종료한 뒤 다시 실행해 주세요."
    exit 1
fi
rm -rf -- "$BASE" "${XDG_CACHE_HOME:-$HOME/.cache}/kakaotalk-linux"
rm -f -- "$HOME/.local/bin/kakaotalk" "$DATA/applications/kakaotalk.desktop" \
    "$DATA/icons/hicolor/256x256/apps/kakaotalk.png"
echo "지웠어요. umu 실행 환경(~/.local/share/umu)은 다른 앱이 쓸 수 있어서 남겨 뒀어요."
