FROM golang:1.25@sha256:dd08f769578a5f51a22bf6a81109288e23cfe2211f051a5c29bd1c05ad3db52a

WORKDIR /echoserver
COPY echoserver.go .
RUN go build -o /bin/echoserver echoserver.go

ENTRYPOINT [ "/bin/echoserver" ]
