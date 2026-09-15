# Build a Debian image with systemd configured to test deb package installation.
# See the `test-packages` make target and associated script for how this image is used.
FROM debian:12-slim@sha256:88200866dfff7ea7f5cbcb6ec7c8a701889efe6fe859fe64d6990e4b07ea4171
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
