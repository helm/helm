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
