#!/bin/bash

set -e

cert_valid() {
    local cert_file="$1"

    if [ ! -f "$cert_file" ]; then
        return 1
    fi

    # Returns success only if certificate is currently valid (not expired).
    openssl x509 -checkend 0 -noout -in "$cert_file" >/dev/null 2>&1 || return 1
}

if test -f ca-cert.pem && test -f ca-key.pem && test -f client-cert.pem && test -f client-key.pem && test -f client-req.pem && test -f server-cert.pem && test -f server-key.pem && test -f server-req.pem; then
    if cert_valid ca-cert.pem && cert_valid server-cert.pem && cert_valid client-cert.pem; then
        exit 0
    fi

    rm -f ca-cert.pem ca-key.pem ca-cert.srl client-cert.pem client-key.pem client-req.pem server-cert.pem server-key.pem server-req.pem
fi

# Generate CA's private key and self-signed certificate
openssl req -x509 -newkey rsa:4096 -days 365 -nodes -keyout ca-key.pem -out ca-cert.pem -subj "/CN=*.deckard.test/emailAddress=server@deckard.test"

# Generate web server's private key and certificate signing request (CSR)
openssl req -newkey rsa:4096 -nodes -keyout server-key.pem -out server-req.pem -subj "/CN=*.deckard.test/emailAddress=server@deckard.test"

# Use CA's private key to sign web server's CSR and get back the signed certificate
openssl x509 -req -in server-req.pem -days 60 -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem -extfile server-ext.cnf

# Generate client's private key and certificate signing request (CSR)
openssl req -newkey rsa:4096 -nodes -keyout client-key.pem -out client-req.pem -subj "/CN=*.deckard-client.test/emailAddress=client@deckard.test"

# Use CA's private key to sign client's CSR and get back the signed certificate
openssl x509 -req -in client-req.pem -days 60 -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out client-cert.pem -extfile client-ext.cnf