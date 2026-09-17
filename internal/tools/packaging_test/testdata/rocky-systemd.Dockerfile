# Build a Rocky Linux image with systemd enabled to test RPM package installation.
# See the `test-packages` make target and associated script for how this image is used.
FROM rockylinux/rockylinux:9@sha256:8101994123cf3d0a8fee517bee7f39e555c7d92bd2d9eb3303cc988a0eeed00f
ENV container=docker
# procps-ng provides ps, used to assert which engine the service launched.
RUN dnf -y install systemd procps-ng \
        && dnf clean all \
        && rm -rf /var/cache/dnf

CMD ["/sbin/init"]
