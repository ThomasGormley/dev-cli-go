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
# Determine which shell config file to use
if [ -n "$ZSH_VERSION" ] || [ "$SHELL" = "/bin/zsh" ]; then
    SHELL_RC="$HOME/.zshrc"
elif [ -n "$BASH_VERSION" ] || [ "$SHELL" = "/bin/bash" ]; then
    SHELL_RC="$HOME/.bashrc"
else
    # Default to .bashrc if we can't determine the shell
    SHELL_RC="$HOME/.bashrc"
fi

# Check if already in PATH (both current PATH and shell config)
if echo "$PATH" | grep -q "$INSTALL_DIR" && grep -q "\$HOME/.dev/bin" "$SHELL_RC" 2>/dev/null; then
    echo "✅ Already in PATH"
elif ! grep -q "\$HOME/.dev/bin" "$SHELL_RC" 2>/dev/null; then
    echo "📝 Adding to PATH in $SHELL_RC..."
    echo "" >> "$SHELL_RC"
    echo "# dev-cli" >> "$SHELL_RC"
    echo "export PATH=\"\$HOME/.dev/bin:\$PATH\"" >> "$SHELL_RC"
    echo "✅ Added to PATH. Restart your shell or run: source $SHELL_RC"
else
    echo "✅ Already configured in shell config"
fi
echo

# Verify and display the installed binary's location and version
"$INSTALL_DIR/$BINARY_NAME" --help
