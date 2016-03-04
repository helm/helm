# Copyright 2016 The Kubernetes Authors All rights reserved.
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
FROM alpine:3.3

COPY . /

# install common packages
RUN apk add --update-cache curl python

WORKDIR /opt

# install pip
RUN curl -sSL https://raw.githubusercontent.com/pypa/pip/7.1.2/contrib/get-pip.py | python -

# install dependencies
RUN pip install --disable-pip-version-check --no-cache-dir -r /opt/expansion/requirements.txt

# cleanup.
RUN apk del --purge \
  curl \
  python \
  && rm -rf /var/cache/apk/*

# define execution environment
CMD ["/bin/expandybird", "-expansion_binary", "/opt/expansion/expansion.py"]
EXPOSE 8000
