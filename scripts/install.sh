#!/bin/bash
set -eo pipefail

# Define the binary name and installation path
BINARY_NAME="dev"
BINARY_LOCATION="bin/$BINARY_NAME"
DEV_INSTALL="$HOME/.dev"
INSTALL_DIR="$DEV_INSTALL/bin"
SCRIPT_DIR=$(dirname "$0")
REPO_DIR=$(dirname "$SCRIPT_DIR")
DIR_BINARY=${REPO_DIR}/bin/${BINARY_NAME}

# Check if the binary exists
if [ ! -f ${DIR_BINARY} ]; then
    echo "❌ Binary '$BINARY_NAME' does not exist."
    echo "❌ Building..."
    go build -C ${REPO_DIR} -o ${DIR_BINARY}
fi

# Create install directory if it doesn't exist
mkdir -p "$INSTALL_DIR"

# Check for write permission to the install directory
if [ ! -w "$INSTALL_DIR" ]; then
    echo "❌ Error: No permission to write to $INSTALL_DIR."
    echo "🔑 Try running this script as root or use sudo."
    exit 2
fi

# Remove old installation if it exists
if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
    echo "🗑️  Removing old installation..."
    rm "$INSTALL_DIR/$BINARY_NAME"
fi

# Copy the binary to the installation directory
cp "$BINARY_LOCATION" "$INSTALL_DIR"
echo
echo "✅ Installed $BINARY_NAME to $INSTALL_DIR successfully."
echo
# Add to shell config
SHELL_RC="$HOME/.zshrc"


# Check if already in PATH
if ! grep -q "$INSTALL_DIR" "$SHELL_RC" 2>/dev/null; then
    echo "📝 Adding to PATH in $SHELL_RC..."
    echo "" >> "$SHELL_RC"
    echo "# dev-cli" >> "$SHELL_RC"
    echo "export DEV_INSTALL=\"$DEV_INSTALL\"" >> "$SHELL_RC"
    echo "export PATH=\"\$DEV_INSTALL/bin:\$PATH\"" >> "$SHELL_RC"
    echo "✅ Added to PATH. Restart your shell or run: source $SHELL_RC"
else
    echo "✅ Already in PATH"
fi
echo

# Verify and display the installed binary's location and version
"$INSTALL_DIR/$BINARY_NAME" --help
