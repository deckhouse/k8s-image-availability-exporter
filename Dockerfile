FROM golang:1.23.0-bullseye@sha256:ecef8303ced05b7cd1addf3c8ea98974f9231d4c5a0c230d23b37bb623714a23 as build

WORKDIR /go/src/app
ADD . /go/src/app

RUN go get -d -v ./...

ARG TAG
RUN CGO_ENABLED=0 go build -a \
    -ldflags "-s -w -extldflags '-static' -X github.com/flant/k8s-image-availability-exporter/pkg/version.Version=${TAG}" \
    -o /go/bin/k8s-image-availability-exporter main.go

FROM gcr.io/distroless/static-debian11@sha256:1dbe426d60caed5d19597532a2d74c8056cd7b1674042b88f7328690b5ead8ed
COPY --from=build /go/bin/k8s-image-availability-exporter /
ENTRYPOINT ["/k8s-image-availability-exporter"]
