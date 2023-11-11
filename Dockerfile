FROM alpine:3.18.4 AS base

RUN addgroup -g 1000 -S helm && adduser -u 1000 -S helm -G helm

#NOSONAR docker:S6596 Sonar bug: virtual scratch image doesn't have any tags, not even :latest
# And Sonar doesn't process "trailing" comments in multi-stage Dockerfiles or parser directives like "# syntax=docker/dockerfile:1":
# https://docs.sonarsource.com/sonarcloud/advanced-setup/languages/docker/#no-nosonar-support
FROM scratch

COPY --chmod=0444 --from=base /etc/passwd /etc/group /etc/
COPY --chmod=0555 --chown=1000:1000 helm /bin/helm

USER helm
WORKDIR /in
WORKDIR /out

ENTRYPOINT ["/bin/helm"]
