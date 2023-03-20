#Build stage
FROM golang:1.20-alpine3.17 AS build-env

ARG GOPROXY
ENV GOPROXY ${GOPROXY:-direct}

ENV CGO_ENABLED=0 

#Build deps
RUN apk --no-cache add build-base git

#Setup repo
COPY . ${GOPATH}/src/gitea.com/gitea/act_runner
WORKDIR ${GOPATH}/src/gitea.com/gitea/act_runner

#Checkout version if set
RUN if [ -n "${ACT_RUNNER_VERSION}" ]; then git checkout "${ACT_RUNNER_VERSION}"; fi \
 && make clean build

FROM alpine:3.6 as alpine

RUN apk add -U --no-cache ca-certificates

FROM scratch
LABEL maintainer="maintainers@gitea.io"

ENV GITEA_RUNNER_FILE="/config/.runner"

COPY --from=alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-env /go/src/gitea.com/gitea/act_runner/act_runner /runner

ENTRYPOINT ["/runner"]
CMD [ "daemon" ]