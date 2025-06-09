FROM golang:1.24.4-bullseye as build

WORKDIR /go/src/app
ADD . /go/src/app

RUN go get -d -v ./...

ARG TAG
RUN CGO_ENABLED=0 go build -a \
    -ldflags "-s -w -extldflags '-static' -X github.com/flant/k8s-image-availability-exporter/pkg/version.Version=${TAG}" \
    -o /go/bin/k8s-image-availability-exporter main.go

FROM gcr.io/distroless/static-debian11
COPY --from=build /go/bin/k8s-image-availability-exporter /
ENTRYPOINT ["/k8s-image-availability-exporter"]
