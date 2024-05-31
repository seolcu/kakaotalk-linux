#!/usr/bin/env bash

# Set up flatpak
sudo flatpak remote-add --if-not-exists flathub https://dl.flathub.org/repo/flathub.flatpakrepo

# Install Bottles
flatpak install flathub com.usebottles.bottles

# Variables
bottles_cli="flatpak run --command=bottles-cli com.usebottles.bottles"
bottle_name="test"

# Create a new bottle
$bottles_cli new --bottle-name $bottle_name --environment application

# Fonts
wget -nc https://github.com/googlefonts/noto-emoji/raw/main/fonts/NotoColorEmoji.ttf
wget -nc https://github.com/googlefonts/noto-cjk/raw/main/Sans/Variable/TTF/NotoSansCJKkr-VF.ttf
cp NotoColorEmoji.ttf NotoSansCJKkr-VF.ttf ~/.var/app/com.usebottles.bottles/data/bottles/bottles/$bottle_name/drive_c/windows/Fonts/

# Run KakaoTalk Installer
wget -nc https://app-pc.kakaocdn.net/talk/win32/under_win10/KakaoTalk_Setup.exe
flatpak override --user --filesystem="$PWD" com.usebottles.bottles
$bottles_cli run -b $bottle_name -e ./KakaoTalk_Setup.exe

# Desktop Entry
flatpak override com.usebottles.bottles --user --filesystem=xdg-data/applications
