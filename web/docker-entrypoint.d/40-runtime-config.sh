#!/bin/sh
set -eu

: "${VITE_API_URL:=http://localhost:8081}"

cat > /usr/share/nginx/html/config.js <<EOF
window.__ENV = { VITE_API_URL: "${VITE_API_URL}" };
EOF
