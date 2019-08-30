############# builder #############
FROM golang:1.12.8 AS builder

WORKDIR /go/src/github.wdf.sap.corp/kubernetes/remedy-controller
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go install \
  -ldflags="-s -w \
    -X github.wdf.sap.corp/kubernetes/remedy-controller/pkg/version.gitVersion=$(cat VERSION) \
    -X github.wdf.sap.corp/kubernetes/remedy-controller/pkg/version.gitCommit=$(git rev-parse --verify HEAD) \
    -X github.wdf.sap.corp/kubernetes/remedy-controller/pkg/version.buildDate=$(date --rfc-3339=seconds | sed 's/ /T/')" \
  -mod=vendor \
  .

############# controller #############
FROM alpine:3.10 AS controller

RUN apk add --update bash curl

COPY --from=builder /go/bin/remedy-controller /remedy-controller
WORKDIR /

ENTRYPOINT ["/remedy-controller"]