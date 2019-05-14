FROM golang:1.12
COPY . /go/src/github.com/wish/kops-channel-exporter/
ENV GO111MODULE=on
WORKDIR /go/src/github.com/wish/kops-channel-exporter
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo .



FROM alpine:3.7
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=0 /go/src/github.com/wish/kops-channel-exporter/kops-channel-exporter /root/exporter
CMD /root/exporter
