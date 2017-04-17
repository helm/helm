FROM ubuntu:latest

RUN apt-get update -y && apt-get install -y openjdk-7-jre tar dnsutils curl

RUN curl -s http://archive.apache.org/dist/storm/apache-storm-0.9.5/apache-storm-0.9.5.tar.gz | \
      tar zxf - -C /opt && \
      mv /opt/apache-storm-0.9.5 /opt/apache-storm

ADD configure.sh /
