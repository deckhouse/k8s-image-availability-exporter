FROM golang:1.16-buster as build

WORKDIR /go/src/app
ADD . /go/src/app

RUN go get -d -v ./...

RUN CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static"' -o /go/bin/k8s-image-availability-exporter main.go

FROM gcr.io/distroless/static-debian10
COPY --from=build /go/bin/k8s-image-availability-exporter /
ENTRYPOINT ["/k8s-image-availability-exporter"]