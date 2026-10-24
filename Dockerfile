# Pinned to the patch go.mod names; dependabot moves both.
FROM golang:1.26.10-alpine AS builder

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

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/yarilo-autoconfig /usr/local/bin/yarilo-autoconfig
USER 65532:65532
EXPOSE 8080/tcp 8081/tcp
ENTRYPOINT ["/usr/local/bin/yarilo-autoconfig"]
