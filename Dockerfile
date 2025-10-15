# syntax=docker/dockerfile:1.7

# ---- build stage ----
ARG GHTTP_BUILDER_IMAGE=golang:1.25
FROM ${GHTTP_BUILDER_IMAGE} AS builder
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
ARG GHTTP_RUNTIME_IMAGE=gcr.io/distroless/base-debian12
FROM ${GHTTP_RUNTIME_IMAGE}
ARG GHTTP_RUNTIME_IMAGE
ARG GHTTP_BUILDER_IMAGE
LABEL org.temirov.ghttp.builder-image=${GHTTP_BUILDER_IMAGE}
LABEL org.temirov.ghttp.runtime-image=${GHTTP_RUNTIME_IMAGE}
WORKDIR /app
COPY --from=builder /out/ghttp /app/ghttp
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/ghttp"]
    
