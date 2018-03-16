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

FROM circleci/golang:1.10

USER root

RUN apt-get update -y && \
  apt-get install -yq zip && \
  apt-get clean

# install gcloud
ARG GCLOUD_VERSION=193.0.0
RUN curl -Lo gcloud.tgz https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-${GCLOUD_VERSION}-linux-x86_64.tar.gz && \
  tar xf gcloud.tgz && \
  google-cloud-sdk/install.sh && \
  rm -rf gcloud.tgz
ENV PATH "/go/google-cloud-sdk/bin:${PATH}"

# install go depenencies
RUN go get -u github.com/Masterminds/glide github.com/mitchellh/gox
