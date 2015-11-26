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

RUN mkdir -p "$GOPATH/src/github.com" && chmod 777 "$GOPATH/src/github.com"

RUN go-wrapper download github.com/kubernetes/deployment-manager/resourcifier/...
RUN go-wrapper install github.com/kubernetes/deployment-manager/resourcifier/...

EXPOSE 8080

ENTRYPOINT ["bin/resourcifier", "--kubectl=/usr/local/bin/kubectl"]
