FROM debian:stretch-slim

RUN apt-get update && apt-get install -y curl ca-certificates procps
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.7.0/bin/linux/amd64/kubectl && mv kubectl /usr/local/bin && chmod 755 /usr/local/bin/kubectl


ENV HELM_HOME=/helmhome
ENV HELM_HOST=127.0.0.1:44134
ENV KUBECONFIG=/kube/config

COPY _functional_tests/run.sh /run.sh
# Copy the scripts
COPY _functional_tests/tests /tests
# Copy the charts
#COPY _functional_tests/tests/chart* /tests/
COPY _dist/linux-amd64/helm /usr/bin/helm
COPY _dist/linux-amd64/tiller /usr/bin/tiller

RUN chmod 755 /tests/*.sh

ENTRYPOINT /run.sh
