# Copyright 2016 The Kubernetes Authors.
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

FROM ubuntu:xenial

RUN apt-get update \
	&& apt-get -y install make curl vim mercurial git dos2unix 

# Install golang using godeb which transforms golang tar.gz archive into deb package
ENV GO_VERSION 1.9
RUN mkdir /tmp/godeb \
	&& curl "https://godeb.s3.amazonaws.com/godeb-amd64.tar.gz" > /tmp/godeb/godeb.tar.gz \
	&& (cd /tmp/godeb; tar zvxf godeb.tar.gz; ./godeb install "${GO_VERSION}") \
	&& rm -rf /tmp/godeb \
	&& mkdir -p /usr/local/go/bin \	

ENV GOPATH /root/go 
ENV PATH "${PATH}:/root/go/bin:/usr/local/go/src/k8s.io/helm/bin"

# Install glide to handle dependencies 
RUN mkdir -p /usr/local/go/src/github.com/Masterminds \
	&& cd /usr/local/go/src/github.com/Masterminds \
	&& git clone https://github.com/Masterminds/glide.git \
	&& cd glide \
	&& make build \
	&& mv glide $GOPATH/bin/glide

# Prepare for work
WORKDIR /usr/local/go/src/k8s.io/helm

ADD . /usr/local/go/src/k8s.io/helm

RUN dos2unix /usr/local/go/src/k8s.io/helm/scripts/*