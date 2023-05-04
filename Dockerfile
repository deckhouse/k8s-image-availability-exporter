FROM golang:1.19-buster as build

WORKDIR /go/src/app
ADD . /go/src/app

RUN go get -d -v ./...

RUN CGO_ENABLED=0 go build -a -ldflags '-s -w -extldflags "-static"' -o /go/bin/k8s-image-availability-exporter main.go
RUN apt update && apt install amazon-ecr-credential-helper  -y

FROM gcr.io/distroless/base

COPY --from=build /go/bin/k8s-image-availability-exporter /
COPY --from=build /usr/bin/docker-credential-ecr-login /usr/bin/docker-credential-ecr-login
ENTRYPOINT ["/k8s-image-availability-exporter"]