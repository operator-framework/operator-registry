#!/usr/bin/env bash

ROOT="$(git rev-parse --show-toplevel)"
DEST="${ROOT}/tools/bin"

ver=$1; shift

ver_cmd="${DEST}/protoc --version 2>/dev/null | cut -d' ' -f2"
os="$(uname -s | sed 's/Darwin/osx/')"
arch="$(uname -m | sed 's/arm64/aarch_64/')"
fetch_cmd="(curl -sSfLo '${DEST}/protoc-${ver}.zip' 'https://github.com/protocolbuffers/protobuf/releases/download/v${ver}/protoc-${ver}-${os}-${arch}.zip' && unzip -o -j -d '${DEST}' '${DEST}/protoc-${ver}.zip' bin/protoc && rm ${DEST}/protoc-${ver}.zip)"

if [[ "${ver}" != "$(eval ${ver_cmd})" ]]; then
  echo "protoc missing or not version '${ver}', downloading..."
  mkdir -p ${DEST}
  eval ${fetch_cmd}
fi

