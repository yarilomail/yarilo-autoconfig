# Bases pinned by digest, the same as the yarilo image; dependabot moves them.
FROM golang:1.27.4-alpine3.24@sha256:0a5f7fe796599ff487605987432e7619ae45beeb2fba4c2ce0b907879932aebf AS builder

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

FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
# Upgrade at build time so a rebuild picks up fixed packages under the same tag.
# hadolint ignore=DL3018,DL3019
RUN apk upgrade --no-cache && adduser -D -H -u 65532 autoconfig
COPY --from=builder /out/yarilo-autoconfig /usr/local/bin/yarilo-autoconfig
USER 65532:65532
EXPOSE 8080/tcp 8081/tcp
ENTRYPOINT ["/usr/local/bin/yarilo-autoconfig"]
