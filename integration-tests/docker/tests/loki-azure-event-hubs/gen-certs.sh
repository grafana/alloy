#!/bin/sh
# Populates /certs with the files the kafka-eh container needs under its
# /etc/kafka/secrets/ dir: a self-signed cert + key (and a concatenated PEM
# keystore that Kafka reads), plus copies of the static JAAS and credentials
# files committed alongside this script. Runs on every cert-init invocation,
# but the cert regeneration step short-circuits while the existing cert is
# still valid. Invoked by the cert-init service in docker-compose.yaml.
set -e

cp /input/kafka_server_jaas.conf /input/keystore.creds /certs/

if [ -f /certs/server.crt ] && openssl x509 -in /certs/server.crt -checkend 86400 -noout 2>/dev/null; then
  echo "cert-init: existing /certs/server.crt is still valid, skipping regeneration"
else
  openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout /certs/server.key \
    -out /certs/server.crt \
    -days 365 \
    -subj '/CN=kafka-eh' \
    -addext 'subjectAltName=DNS:kafka-eh,DNS:localhost,IP:127.0.0.1'

  cat /certs/server.crt /certs/server.key > /certs/kafka-keystore.pem
  echo "cert-init: generated fresh /certs/server.crt + kafka-keystore.pem"
fi

# Chown to the kafka-eh container's runtime uid (1000 / appuser), which also
# matches the typical host user — so files are host-readable and
# `rm -rf ./certs` works without docker gymnastics.
chmod 644 /certs/*
chown 1000:1000 /certs /certs/*
