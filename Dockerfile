#Build stage
FROM golang:1.20-alpine3.17 AS build-env

ARG GOPROXY
ENV GOPROXY ${GOPROXY:-direct}

#Build deps
RUN apk --no-cache add build-base git

#Setup repo
COPY . ${GOPATH}/src/gitea.com/gitea/act_runner
WORKDIR ${GOPATH}/src/gitea.com/gitea/act_runner

#Checkout version if set
RUN if [ -n "${ACT_RUNNER_VERSION}" ]; then git checkout "${ACT_RUNNER_VERSION}"; fi \
 && make clean build

FROM alpine:3.17
LABEL maintainer="maintainers@gitea.io"

ARG INSTANCE
ENV INSTANCE ${INSTANCE:-}

ARG TOKEN
ENV TOKEN ${TOKEN:-}

COPY --from=build-env /go/src/gitea.com/gitea/act_runner/act_runner /app/act_runner/act_runner
COPY --from=build-env /go/src/gitea.com/gitea/act_runner/docker.sh /app/act_runner/runner.sh

ENTRYPOINT ["sh", "/app/act_runner/runner.sh"]
