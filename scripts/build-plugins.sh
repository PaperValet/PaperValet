#!/bin/bash
# Build all external plugins as .so files

set -e

PLUGINS_DIR="/root/PaperValet/plugins-external"
BUILD_DIR="/root/PaperValet/build/plugins"
mkdir -p "$BUILD_DIR"

PLUGINS=("ping" "help" "admin" "status" "exec" "debug" "alias" "prefix" "reload" "sudo" "leech" "tpm")

echo "Building external plugins..."
for plugin in "${PLUGINS[@]}"; do
    PLUGIN_DIR="$PLUGINS_DIR/$plugin"
    if [ -d "$PLUGIN_DIR" ]; then
        echo "Building $plugin..."
        cd "$PLUGIN_DIR"
        if [ -f go.mod ]; then
            go mod tidy 2>/dev/null || true
            if go build -buildmode=plugin -o "$BUILD_DIR/$plugin.so" . 2>/dev/null; then
                echo "  ✓ $plugin.so built successfully"
            else
                echo "  ✗ $plugin failed to build"
            fi
        else
            echo "  ⚠ $plugin has no go.mod, skipping"
        fi
    else
        echo "  ⚠ $plugin directory not found, skipping"
    fi
done

echo ""
echo "Built plugins in $BUILD_DIR:"
ls -la "$BUILD_DIR"/*.so 2>/dev/null || echo "  (none)"