#!/usr/bin/env bash
set -euo pipefail
SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
[[ -n "${DEBUG:-}" ]] && set -x

cd "${SCRIPTDIR}/.."

name="rundevd-v0.0.0-$(git describe --always --dirty)"
bucket="${BUCKET:-rundev-test}"

env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build -o /dev/stdout ./cmd/daemon \
		| gsutil -q cp - gs://"${bucket}/${name}" 1>&2

echo "https://storage.googleapis.com/${bucket}/${name}"
