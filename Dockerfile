############# builder
FROM golang:1.19.5 AS builder

WORKDIR /go/src/github.com/gardener/remedy-controller
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
