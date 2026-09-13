#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="${BACKEND_DIR:-${ROOT_DIR}/../backend}"
SOURCE="${BACKEND_DIR}/protos/alora.proto"
TARGET="${ROOT_DIR}/proto/alora.proto"

if [[ ! -f "${SOURCE}" ]]; then
	printf 'proto manbasi topilmadi: %s\n' "${SOURCE}" >&2
	exit 1
fi

mkdir -p "$(dirname "${TARGET}")"
if [[ -f "${TARGET}" ]]; then
	diff -u "${TARGET}" "${SOURCE}" || true
fi
cp "${SOURCE}" "${TARGET}"
printf 'proto import qilindi: %s -> %s\n' "${SOURCE}" "${TARGET}"
