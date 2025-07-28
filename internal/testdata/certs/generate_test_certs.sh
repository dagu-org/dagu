#!/bin/bash

# Generate test certificates for testing TLS functionality
# These are self-signed certificates and should ONLY be used for testing

set -e

cd "$(dirname "$0")"

# Clean up any existing files
rm -f *.pem *.key *.crt *.csr *.srl

# Generate CA key and certificate
openssl genrsa -out ca-key.pem 2048
openssl req -new -x509 -days 3650 -key ca-key.pem -out ca.pem -subj "/C=US/ST=Test/L=Test/O=DaguTest/CN=Test CA"

# Generate peer certificate and key
openssl genrsa -out key.pem 2048
openssl req -new -key key.pem -out cert.csr -subj "/C=US/ST=Test/L=Test/O=DaguTest/CN=localhost"
openssl x509 -req -in cert.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out cert.pem -days 3650

# Clean up temporary files
rm -f *.csr *.srl

echo "Test certificates generated successfully!"
echo "Files created:"
echo "  - ca.pem (CA certificate)"
echo "  - cert.pem (Peer certificate)"
echo "  - key.pem (Peer key)"