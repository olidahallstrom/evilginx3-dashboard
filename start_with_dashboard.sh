#!/bin/bash

# Evilginx2 with Web Dashboard
# This script starts Evilginx2 with the web dashboard enabled

echo "🕷️  Starting Evilginx2 with Web Dashboard..."
echo "Dashboard will be available at: http://localhost:8080"
echo "Terminal interface will also be available for advanced configuration"
echo ""

# Start evilginx2 with dashboard enabled on port 8080
./evilginx2 -dashboard 8080 "$@" 