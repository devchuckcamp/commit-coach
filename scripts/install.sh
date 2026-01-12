#!/usr/bin/env bash
set -euo pipefail

# commit-coach installer (Bash/Zsh).
# Builds the binary and installs it into a directory on your PATH.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

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

# Pick an install dir.
# Prefer user-local bin to avoid sudo.
DEFAULT_INSTALL_DIR="$HOME/.local/bin"

# On Windows shells (Git Bash/MSYS), default to Program Files (Node.js-like)
# and fall back to user-local bin when not writable.
PREFERRED_WINDOWS_DIR="/c/Program Files/commit-coach"

INSTALL_DIR="${COMMIT_COACH_INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"
if [[ -z "${COMMIT_COACH_INSTALL_DIR:-}" && "$EXE_SUFFIX" == ".exe" ]]; then
  INSTALL_DIR="$PREFERRED_WINDOWS_DIR"
fi

try_prepare_install_dir() {
  local dir="$1"
  if mkdir -p "$dir" 2>/dev/null; then
    # shellcheck disable=SC2091
    if ( : >"$dir/.commit-coach-write-test" ) 2>/dev/null; then
      rm -f "$dir/.commit-coach-write-test" || true
      return 0
    fi
  fi
  return 1
}

if ! try_prepare_install_dir "$INSTALL_DIR"; then
  if [[ "$EXE_SUFFIX" == ".exe" ]]; then
    echo "Warning: cannot write to $INSTALL_DIR"
    echo "Tip: re-run this installer from an elevated terminal (Run as Administrator)"
    echo "Falling back to user install dir: $DEFAULT_INSTALL_DIR"
  fi
  INSTALL_DIR="$DEFAULT_INSTALL_DIR"
  mkdir -p "$INSTALL_DIR"
fi

TMP_DIR="$(mktemp -d)"
cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

BIN_NAME="commit-coach${EXE_SUFFIX}"
OUT_PATH="$TMP_DIR/$BIN_NAME"

echo "Building $BIN_NAME..."
cd "$ROOT_DIR"
go build -o "$OUT_PATH" .

# Also drop a local build artifact for convenience.
DIST_DIR="$ROOT_DIR/dist"
mkdir -p "$DIST_DIR"
cp "$OUT_PATH" "$DIST_DIR/$BIN_NAME"
if [[ "$EXE_SUFFIX" == "" ]]; then
  chmod +x "$DIST_DIR/$BIN_NAME" || true
fi
echo "Built: $DIST_DIR/$BIN_NAME"

# Use dist artifact as the single source of truth for installation.
INSTALL_SRC="$DIST_DIR/$BIN_NAME"

if command -v sha256sum >/dev/null 2>&1; then
	echo "SHA256: $(sha256sum "$INSTALL_SRC" | awk '{print $1}')"
fi

# Ensure executable bit on non-Windows.
if [[ "$EXE_SUFFIX" == "" ]]; then
  chmod +x "$OUT_PATH" || true
fi

DEST="$INSTALL_DIR/$BIN_NAME"

# Install from dist artifact.
cp "$INSTALL_SRC" "$DEST"

echo "Installed: $DEST"

if [[ "$EXE_SUFFIX" == ".exe" ]]; then
  # Git Bash typically doesn't resolve .exe without PATHEXT like cmd.exe does.
  # Create a small shim so users can type `commit-coach`.
  SHIM="$INSTALL_DIR/commit-coach"
  cat >"$SHIM" <<'EOF'
#!/usr/bin/env bash
exec "$(dirname "$0")/commit-coach.exe" "$@"
EOF
  chmod +x "$SHIM" || true
  echo "Installed: $SHIM"
fi

# Check PATH hint.
case ":$PATH:" in
  *":$INSTALL_DIR:"*)
    echo "OK: $INSTALL_DIR is on your PATH"
    ;;
  *)
    echo ""
    echo "Next: add this to your shell profile (e.g. ~/.bashrc or ~/.zshrc):"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    echo "Then restart your terminal."
    ;;
esac

echo ""
echo "Try: commit-coach --help"
