FROM gcr.io/dm-k8s-testing/storm-base

ADD start.sh /

EXPOSE 6700 6701 6702 6703

WORKDIR /opt/apache-storm
ADD google_application_credentials.json ./
ENV GOOGLE_APPLICATION_CREDENTIALS /opt/apache-storm/google_application_credentials.json

ENTRYPOINT ["/start.sh"]
