#!/bin/bash

# Define the binary name and installation path
BINARY_NAME="dev"
BINARY_LOCATION="bin/$BINARY_NAME"
INSTALL_DIR="$HOME/bin"

# Check if the binary exists
if [ ! -f "./bin/$BINARY_NAME" ]; then
    echo "Error: Binary '$BINARY_NAME' does not exist."
    exit 1
fi

# Check for write permission to the install directory
if [ ! -w "$INSTALL_DIR" ]; then
    echo "Error: No permission to write to $INSTALL_DIR."
    echo "Try running this script as root or use sudo."
    exit 2
fi

# Copy the binary to the installation directory
cp "$BINARY_LOCATION" "$INSTALL_DIR"
echo "Installed $BINARY_NAME to $INSTALL_DIR successfully."

# Verify and display the installed binary's location and version
command -v "$BINARY_NAME"
"$INSTALL_DIR/$BINARY_NAME" --version
