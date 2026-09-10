# A minimal container that serves a static Prometheus endpoint and declares a
# cAdvisor application-metrics collector through an image label. cAdvisor reads
# the label, fetches the config from the container filesystem, scrapes the
# endpoint, and exposes test_app_metric as a custom metric.
FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11

RUN mkdir -p /www /etc/cadvisor
COPY metrics /www/metrics
COPY collector.json /etc/cadvisor/collector.json

LABEL io.cadvisor.metric.prometheus="/etc/cadvisor/collector.json"

EXPOSE 8080
# Use the busybox binary directly; the httpd applet serves /www over HTTP.
CMD ["/bin/busybox", "httpd", "-f", "-p", "8080", "-h", "/www"]
