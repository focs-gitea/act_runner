# Use host architecture and crosscompile
# CGO is not required and qemu binfmt doesn't work properly
# on some arches
FROM --platform=${BUILDPLATFORM} golang:latest AS build

WORKDIR /build
COPY . .

ARG GOOS=${TARGETOS}
ARG GOARCH=${TARGETARCH}
ARG GOARM=
ARG CGO_ENABLED=0
ARG VERSION=

RUN go build -ldflags="-s -w -X 'gitea.com/gitea/act_runner/cmd.version=${VERSION}'"

# Grab timezones and SSL certificates
# Architecture doesn't matter here
FROM --platform=${BUILDPLATFORM} alpine:latest AS system

RUN apk add --no-cache \
        tzdata \
        ca-certificates

# Create target architecture
FROM --platform=${TARGETPLATFORM} scratch

COPY --from=system /etc/ssl /etc/ssl
COPY --from=system /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /build/act_runner /runner

WORKDIR /data
VOLUME [ "/data" ]
ENTRYPOINT [ "/runner" ]
CMD [ "daemon" ]
