#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="${BACKEND_DIR:-${ROOT_DIR}/../backend}"
SOURCE="${BACKEND_DIR}/tests/fixtures/calc_cases.json"
TARGET="${ROOT_DIR}/tests/fixtures/calc_cases.json"

if [[ ! -f "${SOURCE}" ]]; then
	printf 'fixture manbasi topilmadi: %s\n' "${SOURCE}" >&2
	exit 1
fi

mkdir -p "$(dirname "${TARGET}")"
cp "${SOURCE}" "${TARGET}"
printf 'fixture sync qilindi: %s -> %s\n' "${SOURCE}" "${TARGET}"
