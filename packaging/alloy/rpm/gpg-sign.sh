#!/usr/bin/env bash

# We are not using fpm's signing functionality because it does not work anymore
# https://github.com/jordansissel/fpm/issues/1626

set -euxo pipefail
shopt -s extglob

# Write GPG key to GPG keyring
printf "%s" "${GPG_PUBLIC_KEY}" > /tmp/gpg-public-key
gpg --import /tmp/gpg-public-key
printf "%s" "${GPG_PRIVATE_KEY}" | gpg --import --no-tty --batch --yes --passphrase "${GPG_PASSPHRASE}"

rpm --import /tmp/gpg-public-key

echo "%_gpg_name Grafana Labs <engineering@grafana.com>
%_signature gpg
%_gpg_path /root/.gnupg
%_gpgbin /usr/bin/gpg
%_gpg_digest_algo sha256
%_binary_filedigest_algorithm sha256
%_source_filedigest_algorithm sha256
%__gpg /usr/bin/gpg
%__gpg_sign_cmd     %{__gpg} \
         gpg --no-tty --batch --yes --no-verbose --no-armor \
         --passphrase ${GPG_PASSPHRASE} \
         --pinentry-mode loopback \
         %{?_gpg_digest_algo:--digest-algo %{_gpg_digest_algo}} \
         --no-secmem-warning \
         -u \"%{_gpg_name}\" -sbo %{__signature_filename} %{__plaintext_filename}
" > ~/.rpmmacros

for f in dist/*.rpm; do
  rpm --addsign  "${f}"
  rpm --checksig "${f}"
done
