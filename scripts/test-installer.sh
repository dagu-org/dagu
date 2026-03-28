#!/usr/bin/env bash
# Copyright (C) 2026 Yota Hamada
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALLER="${SCRIPT_DIR}/installer.sh"

fixture_dir="$(mktemp -d)"
trap 'rm -rf "${fixture_dir}"' EXIT

bash -c '
    set -euo pipefail

    installer_path="$1"
    set --
    source "${installer_path}"

    mktempdir tmpdir
    mktempfile tmpfile

    [[ -d "${tmpdir}" ]]
    [[ -f "${tmpfile}" ]]
    [[ "${#TMPFILES[@]}" -ge 2 ]]

    cleanup_tmpfiles

    [[ ! -e "${tmpdir}" ]]
    [[ ! -e "${tmpfile}" ]]
' bash "${INSTALLER}"

bootstrap_file="${fixture_dir}/bootstrap.env"
cat >"${bootstrap_file}" <<'EOF'
DAGU_AUTH_BUILTIN_INITIAL_ADMIN_USERNAME=admin
DAGU_AUTH_BUILTIN_INITIAL_ADMIN_PASSWORD=supersecret
EOF

bash -c '
    set -euo pipefail

    installer_path="$1"
    bootstrap_path="$2"
    set --
    source "${installer_path}"

    OS="linux"
    SERVICE_SCOPE="user"
    SERVICE_BOOTSTRAP_FILE="${bootstrap_path}"

    backup_bootstrap_file backup

    [[ -f "${backup}" ]]
    grep -Fqx "DAGU_AUTH_BUILTIN_INITIAL_ADMIN_USERNAME=admin" "${backup}"
    grep -Fqx "DAGU_AUTH_BUILTIN_INITIAL_ADMIN_PASSWORD=supersecret" "${backup}"
    [[ "${#TMPFILES[@]}" -ge 1 ]]

    cleanup_tmpfiles

    [[ ! -e "${backup}" ]]
' bash "${INSTALLER}" "${bootstrap_file}"

printf '%s\n' "installer regression tests passed"
