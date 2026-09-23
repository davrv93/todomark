#!/bin/sh
set -eu

: "${PUBLIC_API_URL:=http://localhost:8081}"

cat > /usr/share/nginx/html/config.js <<EOF
window.__ENV = { PUBLIC_API_URL: "${PUBLIC_API_URL}" };
EOF
