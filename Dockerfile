# syntax=docker/dockerfile:1.7

# ---- build stage ----
FROM golang:1.25 AS builder
WORKDIR /src

# Buildx will set these automatically per target platform
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

ENV CGO_ENABLED=0
ENV GOFLAGS=-trimpath

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Optional: print target for debugging
RUN echo "Building for ${TARGETOS}/${TARGETARCH}${TARGETVARIANT}"

# Build the binary for the requested platform
RUN --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
        -ldflags="-s -w" \
        -o /out/ghttp ./cmd/ghttp

# ---- runtime stage ----
FROM gcr.io/distroless/base-debian12
LABEL org.temirov.ghttp.builder-image=golang:1.25
LABEL org.temirov.ghttp.runtime-image=gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /out/ghttp /app/ghttp
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/ghttp"]
    
