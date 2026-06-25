# syntax=dockerfile
ARG GO_VERSION=1.26

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY api/ api/
COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 \
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