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

#Build the binary
RUN make clean build

FROM alpine:3.17 as cacerts

RUN apk add -U --no-cache ca-certificates

FROM scratch
LABEL maintainer="maintainers@gitea.io"

ENV GITEA_RUNNER_FILE="/config/.runner"

COPY --from=cacerts /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-env /go/src/gitea.com/gitea/act_runner/act_runner /runner

ENTRYPOINT ["/runner"]
CMD [ "daemon" ]