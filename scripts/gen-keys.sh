#!/usr/bin/env bash
set -e

mkdir -p keys

echo "→ Generating RSA private key..."
openssl genrsa -out keys/private.pem 4096

echo "→ Extracting public key..."
openssl rsa -in keys/private.pem -pubout -out keys/public.pem

echo "✓ Keys generated in keys/"
