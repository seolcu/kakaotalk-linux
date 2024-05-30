#!/usr/bin/env bash

# Install Bottles
./install-bottles.sh

bottles_cli="flatpak run --command=bottles-cli com.usebottles.bottles"

$bottles_cli --help