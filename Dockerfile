# Bases pinned by digest, the same as the yarilo image; dependabot moves them.
FROM golang:1.26.10-alpine@sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308 AS builder

# A newer go.mod must fail the build, not fetch a toolchain.
ENV GOTOOLCHAIN=local
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
ARG BUILD_VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=${BUILD_VERSION}" -trimpath \
    -o /out/yarilo-autoconfig .

FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a
# Upgrade at build time so a rebuild picks up fixed packages under the same tag.
# hadolint ignore=DL3018,DL3019
RUN apk upgrade --no-cache && adduser -D -H -u 65532 autoconfig
COPY --from=builder /out/yarilo-autoconfig /usr/local/bin/yarilo-autoconfig
USER 65532:65532
EXPOSE 8080/tcp 8081/tcp
ENTRYPOINT ["/usr/local/bin/yarilo-autoconfig"]
