# Build a Debian image with systemd configured to test deb package installation.
# See the `test-packages` make target and associated script for how this image is used.
FROM debian:12-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251
ENV container=docker
ENV LC_ALL=C
ENV DEBIAN_FRONTEND=noninteractive
# procps provides ps, used to assert which engine the service launched in testing
RUN apt-get update \
        && apt-get install -y systemd procps \
        && apt-get clean \
        && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
RUN rm -f /lib/systemd/system/multi-user.target.wants/* \
        /etc/systemd/system/*.wants/* \
        /lib/systemd/system/local-fs.target.wants/* \
        /lib/systemd/system/sockets.target.wants/*udev* \
        /lib/systemd/system/sockets.target.wants/*initctl* \
        /lib/systemd/system/sysinit.target.wants/systemd-tmpfiles-setup* \
        /lib/systemd/system/systemd-update-utmp*

CMD ["/lib/systemd/systemd"]
