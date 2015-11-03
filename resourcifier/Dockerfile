# Copyright 2015 Google, Inc. All Rights Reserved
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:1.4
MAINTAINER Jack Greenfield <jackgr@google.com>

WORKDIR /usr/local/bin

RUN apt-get update \
  && apt-get install -y wget

ENV KUBE_PACKAGE kubernetes.tar.gz
ENV KUBE_VERSION v1.0.5
ENV KUBE_BASE https://github.com/kubernetes/kubernetes
ENV KUBE_RELEASE "$KUBE_BASE"/releases/download
ENV KUBE_DOWLOAD "$KUBE_RELEASE"/"$KUBE_VERSION"/"$KUBE_PACKAGE"
ENV KUBE_COMMAND kubernetes/platforms/linux/amd64/kubectl
RUN wget -O - "$KUBE_DOWLOAD" 2> /dev/null | tar xzf - -C /tmp "$KUBE_COMMAND" \
  && mv /tmp/"$KUBE_COMMAND" . \
  && rm -rf /tmp/kubernetes

RUN apt-get purge -y --auto-remove wget

WORKDIR "$GOPATH"

RUN mkdir -p src/github.com/kubernetes/deployment-manager \
  && chmod 777 src/github.com/kubernetes/deployment-manager

RUN mkdir -p src/github.com/kubernetes/deployment-manager/expandybird \
  && chmod 777 src/github.com/kubernetes/deployment-manager/expandybird
COPY expandybird src/github.com/kubernetes/deployment-manager/expandybird

RUN mkdir -p src/github.com/kubernetes/deployment-manager/resourcifier \
  && chmod 777 src/github.com/kubernetes/deployment-manager/resourcifier
COPY resourcifier src/github.com/kubernetes/deployment-manager/resourcifier

RUN mkdir -p src/github.com/kubernetes/deployment-manager/util \
  && chmod 777 src/github.com/kubernetes/deployment-manager/util
COPY util src/github.com/kubernetes/deployment-manager/util

RUN mkdir -p src/github.com/kubernetes/deployment-manager/version \
  && chmod 777 src/github.com/kubernetes/deployment-manager/version
COPY version src/github.com/kubernetes/deployment-manager/version

RUN go-wrapper download github.com/kubernetes/deployment-manager/resourcifier/...
RUN go-wrapper install github.com/kubernetes/deployment-manager/resourcifier/...

EXPOSE 8080

ENTRYPOINT ["bin/resourcifier", "--kubectl=/usr/local/bin/kubectl"]
