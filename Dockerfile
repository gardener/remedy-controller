############# builder
FROM golang:1.14.4 AS builder

WORKDIR /go/src/github.com/gardener/remedy-controller
COPY . .
RUN make install

############# base image
FROM alpine:3.12.0 AS base

############# remedy-controller-azure
FROM base AS remedy-controller-azure

#COPY charts /charts
COPY --from=builder /go/bin/remedy-controller-azure /remedy-controller-azure
ENTRYPOINT ["/remedy-controller-azure"]

############# remedy-applier-azure
FROM base AS remedy-applier-azure

COPY --from=builder /go/bin/remedy-applier-azure /remedy-applier-azure
ENTRYPOINT ["/remedy-applier-azure"]
