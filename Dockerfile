############# builder
FROM golang:1.22.4 AS builder

WORKDIR /go/src/github.com/gardener/remedy-controller
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make install

############# base image
FROM gcr.io/distroless/static-debian11:nonroot AS base

############# remedy-controller-azure
FROM base AS remedy-controller-azure
WORKDIR /

COPY --from=builder /go/bin/remedy-controller-azure /remedy-controller-azure
ENTRYPOINT ["/remedy-controller-azure"]

############# remedy-applier-azure
FROM base AS remedy-applier-azure
WORKDIR /

COPY --from=builder /go/bin/remedy-applier-azure /remedy-applier-azure
ENTRYPOINT ["/remedy-applier-azure"]
