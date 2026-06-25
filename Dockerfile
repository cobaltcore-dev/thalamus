# syntax=docker/dockerfile:1
ARG GO_VERSION=1.26

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY api/ api/
COPY cmd/ cmd/
COPY internal/ internal/

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/operator \
      ./cmd/operator


FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/operator /operator

ENTRYPOINT ["/operator"]


FROM gcr.io/distroless/static-debian12:debug-nonroot AS debug

COPY --from=build /out/operator /operator

ENTRYPOINT ["/operator"]
