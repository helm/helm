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

RUN apt-get update \
    && apt-get autoremove -y \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /usr/local/bin
ENV KUBE_VERSION v1.0.5

RUN curl -fsSL -o kubectl https://storage.googleapis.com/kubernetes-release/release/$KUBE_VERSION/bin/linux/amd64/kubectl \
    && chmod +x kubectl

COPY . "$GOPATH"/src

WORKDIR "$GOPATH"/src/manager

RUN go-wrapper download
RUN go-wrapper install

RUN cp "$GOPATH"/bin/manager /usr/local/bin
RUN /bin/rm -rf "$GOPATH"

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/manager", "--kubectl=/usr/local/bin/kubectl"]
