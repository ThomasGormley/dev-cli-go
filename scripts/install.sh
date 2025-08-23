#!/bin/bash
set -eo pipefail

# Define the binary name and installation path
BINARY_NAME="dev"
BINARY_LOCATION="bin/$BINARY_NAME"
INSTALL_DIR="/usr/local/bin"
SCRIPT_DIR=$(dirname "$0")
REPO_DIR=$(dirname "$SCRIPT_DIR")
DIR_BINARY=${REPO_DIR}/bin/${BINARY_NAME}

# Check if the binary exists
if [ ! -f ${DIR_BINARY} ]; then
    echo "❌ Binary '$BINARY_NAME' does not exist."
    echo "❌ Building..."
    go build -C ${REPO_DIR} -o ${DIR_BINARY}
fi

# Check for write permission to the install directory
if [ ! -w "$INSTALL_DIR" ]; then
    echo "❌ Error: No permission to write to $INSTALL_DIR."
    echo "🔑 Try running this script as root or use sudo."
    exit 2
fi

# Copy the binary to the installation directory
cp "$BINARY_LOCATION" "$INSTALL_DIR"
echo
echo "✅ Installed $BINARY_NAME to $INSTALL_DIR successfully."
echo

# Verify and display the installed binary's location and version
"$INSTALL_DIR/$BINARY_NAME" --help
