FROM golang:1.25@sha256:1c91b4f4391774a73d6489576878ad3ff3161ebc8c78466ec26e83474855bfcf

WORKDIR /echoserver
COPY echoserver.go .
RUN go build -o /bin/echoserver echoserver.go

ENTRYPOINT [ "/bin/echoserver" ]
