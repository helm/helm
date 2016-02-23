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

FROM python:2
MAINTAINER Jack Greenfield <jackgr@google.com>

RUN ln -s /usr/local/bin/python /usr/bin/python

RUN apt-get update \
    && apt-get autoremove -y \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# following lines copied from golang:1.6

RUN apt-get update && apt-get install -y --no-install-recommends \
        g++ \
        gcc \
        libc6-dev \
        make \
    && rm -rf /var/lib/apt/lists/*

ENV GOLANG_VERSION 1.6
ENV GOLANG_DOWNLOAD_URL https://golang.org/dl/go$GOLANG_VERSION.linux-amd64.tar.gz
ENV GOLANG_DOWNLOAD_SHA256 5470eac05d273c74ff8bac7bef5bad0b5abbd1c4052efbdbc8db45332e836b0b

RUN curl -fsSL "$GOLANG_DOWNLOAD_URL" -o golang.tar.gz \
    && echo "$GOLANG_DOWNLOAD_SHA256  golang.tar.gz" | sha256sum -c - \
    && tar -C /usr/local -xzf golang.tar.gz \
    && rm golang.tar.gz

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"

#end copied lines

COPY . "$GOPATH"/src/github.com/kubernetes/deployment-manager
WORKDIR "$GOPATH"/src/github.com/kubernetes/deployment-manager/expandybird

RUN go get -v -d .
RUN go install -v .

RUN mkdir -p /var/expandybird/expansion
WORKDIR /var/expandybird

COPY ./expandybird/expansion /var/expandybird/expansion

COPY ./expandybird/requirements.txt /var/expandybird/requirements.txt
RUN pip install --no-cache-dir -r /var/expandybird/requirements.txt

RUN cp "$GOPATH"/bin/expandybird /var/expandybird/expandybird
RUN /bin/rm -rf "$GOPATH"

EXPOSE 8080

ENTRYPOINT ["/var/expandybird/expandybird", "-expansion_binary", "/var/expandybird/expansion/expansion.py"]
