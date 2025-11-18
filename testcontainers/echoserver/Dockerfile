FROM golang:1.25@sha256:f60eaa87c79e604967c84d18fd3b151b3ee3f033bcdade4f3494e38411e60963

WORKDIR /echoserver
COPY echoserver.go .
RUN go build -o /bin/echoserver echoserver.go

ENTRYPOINT [ "/bin/echoserver" ]
