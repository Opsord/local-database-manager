#!/usr/bin/env bash
set -e

# 1. Resolve repository root dynamically
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_PATH="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Verify repository directory
if [[ ! -d "${REPO_PATH}/engines" || ! -d "${REPO_PATH}/instances" ]]; then
    echo "Error: '${REPO_PATH}' does not appear to be a valid local-database-manager repository." >&2
    exit 1
fi

echo "Using repository path: ${REPO_PATH}"

# 2. Determine binary name and build if missing
BIN_NAME="db-manager"
if [[ "$OSTYPE" == "msys"* || "$OSTYPE" == "cygwin"* || "$OSTYPE" == "win32"* ]]; then
    BIN_NAME="db-manager.exe"
fi

if [[ ! -f "${REPO_PATH}/${BIN_NAME}" ]]; then
    echo "Binary ${BIN_NAME} not found. Building with Go..."
    (cd "${REPO_PATH}" && go build -o "${BIN_NAME}" ./cmd/db-manager)
    echo "Successfully built ${BIN_NAME}!"
else
    echo "Found compiled binary: ${REPO_PATH}/${BIN_NAME}"
fi

# 3. Detect shell profile
PROFILE_FILE="${HOME}/.bashrc"
if [[ -n "${ZSH_VERSION:-}" || "${SHELL:-}" == *"zsh"* ]]; then
    PROFILE_FILE="${HOME}/.zshrc"
fi

# 4. Construct function block with markers
MARKER_START="# >>> local-database-manager (dbm) >>>"
MARKER_END="# <<< local-database-manager (dbm) <<<"

read -r -d '' FUNCTION_BLOCK <<EOF || true
${MARKER_START}
dbm() {
    pushd "${REPO_PATH}" > /dev/null || return 1
    ./${BIN_NAME} "\$@"
    local exit_code=\$?
    popd > /dev/null || return 1
    return \${exit_code}
}
${MARKER_END}
EOF

# 5. Insert or replace idempotently
touch "${PROFILE_FILE}"

if grep -qF "${MARKER_START}" "${PROFILE_FILE}"; then
    # Replace existing block between markers
    python3 -c "
import sys, re
start, end = re.escape('${MARKER_START}'), re.escape('${MARKER_END}')
pattern = rf'{start}[\s\S]*?{end}'
with open('${PROFILE_FILE}', 'r') as f:
    c = f.read()
c = re.sub(pattern, '''${FUNCTION_BLOCK}''', c)
with open('${PROFILE_FILE}', 'w') as f:
    f.write(c)
" 2>/dev/null || {
        # Fallback if python3 not available: remove lines between markers and append
        sed -i "/${MARKER_START}/,/${MARKER_END}/d" "${PROFILE_FILE}"
        echo "${FUNCTION_BLOCK}" >> "${PROFILE_FILE}"
    }
    echo "Updated existing 'dbm' function in ${PROFILE_FILE}"
else
    echo "" >> "${PROFILE_FILE}"
    echo "${FUNCTION_BLOCK}" >> "${PROFILE_FILE}"
    echo "Added 'dbm' shortcut function to ${PROFILE_FILE}"
fi

echo "Setup complete! Run 'source ${PROFILE_FILE}' or open a new terminal, then type 'dbm'."
