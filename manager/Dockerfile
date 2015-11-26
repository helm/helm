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

RUN mkdir -p "$GOPATH/src/github.com" && chmod 777 "$GOPATH/src/github.com"

WORKDIR "$GOPATH/src/github.com"

RUN mkdir -p manager && chmod 777 manager
COPY manager manager

RUN mkdir -p util && chmod 777 util
COPY util util

RUN mkdir -p version && chmod 777 version
COPY version version

WORKDIR "$GOPATH"

RUN mkdir -p "$GOPATH/src/github.com" && chmod 777 "$GOPATH/src/github.com"

RUN go-wrapper download github.com/kubernetes/deployment-manager/manager/...
RUN go-wrapper install github.com/kubernetes/deployment-manager/manager/...

EXPOSE 8080

ENTRYPOINT ["bin/manager"]
