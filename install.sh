#!/usr/bin/env sh
set -eu

# MioNeko user-scope installer for macOS/Linux.
# Run from the extracted release folder. sudo is not required.

INSTALL_DIR="$HOME/.local/bin"
TARGET="$INSTALL_DIR/mimoneko"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

echo "MioNeko macOS/Linux installer"
echo

SOURCE=""
if [ -f "$SCRIPT_DIR/mimoneko" ]; then
  SOURCE="$SCRIPT_DIR/mimoneko"
else
  for candidate in "$SCRIPT_DIR"/mimoneko-*; do
    if [ -f "$candidate" ] && [ -x "$candidate" ]; then
      SOURCE="$candidate"
      break
    fi
  done
fi

if [ -z "$SOURCE" ]; then
  echo "Could not find mimoneko in $SCRIPT_DIR" >&2
  exit 1
fi

mkdir -p "$INSTALL_DIR"
cp "$SOURCE" "$TARGET"
chmod 755 "$TARGET"
echo "Installed: $TARGET"

case ":$PATH:" in
  *":$INSTALL_DIR:"*)
    echo "PATH already contains: $INSTALL_DIR"
    ;;
  *)
    echo
    echo "$INSTALL_DIR is not in PATH."
    echo "Add this line to your shell profile, then reopen the terminal:"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    ;;
esac

echo
echo "Installation complete."
echo "Open a new terminal, then run:"
echo "  mimoneko"
