#!/usr/bin/env bash
set -o errexit

# create temp directory and setup for removal on script exit
TMPDIR="$(mktemp -d)"
MKCERT_TEMP_DIR="${TMPDIR}/mkcert"
trap 'rm -rf -- "$TMPDIR"' EXIT

# output file locations
CERT_DIR=certs

# skip TLS flag (default to using TLS)
SKIP_TLS=false

function usage() {
  echo -n "$(basename "$0") [OPTION]... <registry name>

Creates a named docker registry and optionally generates and uses TLS certs.
TLS uses port 443 and non-TLS uses port 5000.

  <registry name>  name of target regsitry (e.g. cp.stg.icr.io/cp)
  
 Options:
  -s, --skip-tls   Skip TLS certificate generation and usage in image registry (defaults to using TLS if not provided)
  -c, --cert-dir   Output directory for cert and key files (defaults to ./certs)
  -h, --help       Display this help and exit
"
  exit 0
}

function err_exit() {
    echo >&2 "[ERROR] $1"
    exit 1
}

while [[ $# -gt 0 ]]
do
key="$1"

case $key in
    -s|--skip-tls)
    SKIP_TLS=true
    shift
    ;;
    -c|--cert-dir)
    CERT_DIR="$2"
    shift 2
    ;;
    -h|--help)
    usage
    shift
    ;;
    --) # end argument parsing
    shift
    break
    ;;
    --*=|-*) # unsupported flags
    err_exit "Unsupported flag $1"
    ;;
    *)
    POSITIONAL+=("$1") # save it in an array for later
    shift # past argument
    ;;
esac
done
set -- "${POSITIONAL[@]}" # restore positional parameters

REGISTRY_NAME=${1?' Missing required registry name'}

CERT="${CERT_DIR}/cert.pem"
KEY="${CERT_DIR}/key.pem"

function installMkcert() {
  # see if mkcert is already installed
  if ! "$(go env GOPATH)/bin/mkcert" --version &>/dev/null; then
    mkdir -p "${MKCERT_TEMP_DIR}"
    # not installed, so clone the repo to temp dir
    if ! git clone https://github.com/FiloSottile/mkcert "${MKCERT_TEMP_DIR}"; then
      err_exit "unable to clone mkcert git repository"
    fi

    # use sub shell to prevent changing users CWD
    if (cd "${MKCERT_TEMP_DIR}" && go install -ldflags "-X main.Version=$(git describe --tags)"); then
      # run mkcert
      if ! "$(go env GOPATH)/bin/mkcert" --version &>/dev/null; then
        err_exit "mkcert is not functioning correctly"
      fi
    else
      err_exit "unable to install mkcert"
    fi
  fi
}

function createCerts() {
  # create destination for certs
  mkdir -p "${CERT_DIR}"
  # check if CAROOT is installed or not
  if [[ ! -d $("$(go env GOPATH)/bin/mkcert" -CAROOT) ]]; then
    if ! "$(go env GOPATH)/bin/mkcert" -install; then
      err_exit "Unable to install CAROOT"
    fi
  fi
  # check if the certs were already created from a previous run of this script
  if [[ ! -f "${CERT}" ]] && [[ ! -f "${KEY}" ]]; then
    if ! "$(go env GOPATH)/bin/mkcert" -cert-file "${CERT}" -key-file "${KEY}" localhost 127.0.0.1 ::1; then
      err_exit "Unable to create cert and key files"
    fi
  fi
}

running="$(docker inspect -f '{{.State.Running}}' "${REGISTRY_NAME}" 2>/dev/null || true)"
if [ "${running}" = 'true' ]; then
  echo "Registry is already running. Nothing to do."
  exit 0
fi

if [[ "$SKIP_TLS" = false ]] ; then
  # Configured to use TLS

  # use mkcert tool for simple cross platform setup/configuration
  installMkcert
  createCerts

  reg_port='443'
  absolute_path_cert_dir="$(cd "$(dirname "${CERT_DIR}")"; pwd -P)/$(basename "${CERT_DIR}")"
  docker run \
    -d \
    --restart=always \
    --name "${REGISTRY_NAME}" \
    -v "${absolute_path_cert_dir}":/certs \
    -e REGISTRY_HTTP_ADDR=0.0.0.0:443 \
    -e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/cert.pem \
    -e REGISTRY_HTTP_TLS_KEY=/certs/key.pem \
    -p "127.0.0.1:${reg_port}:443" \
    registry:2

else
  # Configured to not use TLS

  reg_port='5000'
  docker run \
    -d \
    --restart=always \
    -p "127.0.0.1:${reg_port}:5000" \
    --name "${REGISTRY_NAME}" \
    registry:2
fi