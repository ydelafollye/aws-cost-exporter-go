ARG ARCH="amd64"
ARG OS="linux"
FROM quay.io/prometheus/busybox-${OS}-${ARCH}:latest

ARG ARCH="amd64"
ARG OS="linux"
COPY .build/${OS}-${ARCH}/aws-cost-exporter /bin/aws-cost-exporter

COPY config.example.yaml /etc/aws-cost-exporter/config.yaml

EXPOSE 9000

USER nobody
ENTRYPOINT ["/bin/aws-cost-exporter"]
