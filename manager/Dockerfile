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

WORKDIR /go/src

RUN mkdir -p manager
COPY manager manager

RUN mkdir -p util
COPY util util

RUN mkdir -p version
COPY version version

RUN go-wrapper download manager/...
RUN go-wrapper install manager/...

EXPOSE 8080

ENTRYPOINT ["go-wrapper", "run"]
