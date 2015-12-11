# Copyright 2015 The Kubernetes Authors All rights reserved.
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

FROM golang:1.4-cross

ENV GOLANG_CROSSPLATFORMS \
	darwin/amd64 \
	freebsd/386 freebsd/amd64 freebsd/arm \
	linux/386 linux/amd64 linux/arm \
	windows/386 windows/amd64

RUN mkdir -p "$GOPATH/src/github.com" && chmod -R 777 "$GOPATH/src/github.com"

COPY . "$GOPATH"/src/dm

WORKDIR "$GOPATH"/src/dm

RUN go-wrapper download
RUN set -ex \
    && for platform in $GOLANG_CROSSPLATFORMS; do \
		GOOS=${platform%/*} \
		GOARCH=${platform##*/} \
		go build -v -o dm-${platform%/*}-${platform##*/}; \
    done
