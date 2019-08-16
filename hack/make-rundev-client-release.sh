#!/usr/bin/env bash
set -euo pipefail
SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
[[ -n "${DEBUG:-}" ]] && set -x

cd "${SCRIPTDIR}/.."

bucket="${BUCKET:-rundev-test}"
subpath="${SUBPATH:-nightly/client}"
file="rundev-$(date -u +%Y-%m-%d-%H%M%S)-$(git describe --always --dirty)"
file_latest="rundev-latest"

build_dir="$(mktemp -d)"
trap 'rm -rf -- "${build_dir}"' EXIT

for os in darwin linux; do
  echo >&2 "building $os"
  fp="${bucket}/${subpath}/${os}/${file}"
  fp_latest="${bucket}/${subpath}/${os}/${file_latest}"

  GOOS="${os}" GOARCH="amd64" go build -o "${build_dir}/out" ./cmd/client
  echo >&2 "uploading ${os}"
	gsutil -q cp "${build_dir}/out" gs://"${fp}" 1>&2
	gsutil -q cp "${build_dir}/out" gs://"${fp_latest}" 1>&2

	echo "-> https://storage.googleapis.com/${fp}"
	echo "-> https://storage.googleapis.com/${fp_latest}"
done
