FROM golang:1.23-alpine AS builder

# Do not remove `git` here, it is required for getting runner version when executing `make build`
RUN apk add --no-cache make git

ARG GOPROXY
ENV GOPROXY=${GOPROXY:-}

COPY . /opt/src/act_runner
WORKDIR /opt/src/act_runner

RUN make clean && make build

FROM alpine
RUN apk add --no-cache tini bash git

COPY --from=builder /opt/src/act_runner/act_runner /usr/local/bin/act_runner
COPY scripts/run.sh /usr/local/bin/run.sh

VOLUME /var/run/docker.sock

VOLUME /data

ENTRYPOINT ["/sbin/tini","--","run.sh"]
