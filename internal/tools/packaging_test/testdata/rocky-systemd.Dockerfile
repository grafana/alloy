# Build a Rocky Linux image with systemd enabled to test RPM package installation.
# See the `test-packages` make target and associated script for how this image is used.
FROM rockylinux/rockylinux:9@sha256:bb5755f96bec05ff26165e779ec843d7167544be9e50770508f307019a6d05c4
ENV container=docker
# procps-ng provides ps, used to assert which engine the service launched.
RUN dnf -y install systemd procps-ng \
        && dnf clean all \
        && rm -rf /var/cache/dnf

CMD ["/sbin/init"]
