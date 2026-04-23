#!/bin/sh
# Generates a self-signed cert + key into /certs and writes a Kafka-friendly
# concatenated PEM keystore. Skips regeneration if the existing cert is still
# valid for at least one more day. Invoked by the cert-init service in
# docker-compose.yaml.
set -e

if [ -f /certs/server.crt ] && openssl x509 -in /certs/server.crt -checkend 86400 -noout 2>/dev/null; then
  echo "cert-init: existing /certs/server.crt is still valid, skipping regeneration"
  exit 0
fi

openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout /certs/server.key \
  -out /certs/server.crt \
  -days 365 \
  -subj '/CN=kafka-eh' \
  -addext 'subjectAltName=DNS:kafka-eh,DNS:localhost,IP:127.0.0.1'

cat /certs/server.crt /certs/server.key > /certs/kafka-keystore.pem
chmod 644 /certs/server.key /certs/server.crt /certs/kafka-keystore.pem
echo "cert-init: generated fresh /certs/server.crt + kafka-keystore.pem"
