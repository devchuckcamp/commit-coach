#!/usr/bin/env bash
set -euo pipefail

# commit-coach uninstaller.

OS_UNAME="$(uname -s | tr '[:upper:]' '[:lower:]')"
EXE_SUFFIX=""
case "$OS_UNAME" in
  mingw*|msys*|cygwin*)
    EXE_SUFFIX=".exe"
    ;;
  *)
    EXE_SUFFIX=""
    ;;
esac

DEFAULT_INSTALL_DIR="$HOME/.local/bin"
PREFERRED_WINDOWS_DIR="/c/Program Files/commit-coach"

INSTALL_DIR="${COMMIT_COACH_INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"
if [[ -z "${COMMIT_COACH_INSTALL_DIR:-}" && "$EXE_SUFFIX" == ".exe" ]]; then
  INSTALL_DIR="$PREFERRED_WINDOWS_DIR"
fi

BIN_NAME="commit-coach${EXE_SUFFIX}"
DEST="$INSTALL_DIR/$BIN_NAME"

SHIM="$INSTALL_DIR/commit-coach"

removed_any=false

remove_file() {
  local p="$1"
  if [[ -f "$p" ]]; then
    if rm -f "$p" 2>/dev/null; then
      echo "Removed: $p"
      removed_any=true
      return 0
    fi

    echo "Failed to remove: $p"
    if [[ "$EXE_SUFFIX" == ".exe" && "$p" == /c/Program\ Files/* ]]; then
      echo "Permission denied. Re-run from an elevated terminal (Run as Administrator), or delete it via File Explorer with admin permission."
    fi
    return 1
  fi
  return 2
}

status=0

# Primary location
remove_file "$DEST" || status=$?
remove_file "$SHIM" || true

# If default Windows dir wasn't used or removal failed, try user-local bin as fallback.
if [[ -z "${COMMIT_COACH_INSTALL_DIR:-}" && "$EXE_SUFFIX" == ".exe" ]]; then
  ALT_DIR="$DEFAULT_INSTALL_DIR"
  ALT_DEST="$ALT_DIR/$BIN_NAME"
  ALT_SHIM="$ALT_DIR/commit-coach"
  remove_file "$ALT_DEST" || true
  remove_file "$ALT_SHIM" || true
fi

if [[ "$removed_any" != true ]]; then
  if [[ -f "$DEST" || -f "$SHIM" ]]; then
    # Found but couldn't remove.
    exit 1
  fi
  echo "Not found: $DEST"
  exit 0
fi

exit 0
