FROM golang:1.21@sha256:4746d26432a9117a5f58e95cb9f954ddf0de128e9d5816886514199316e4a2fb

WORKDIR /echoserver
COPY echoserver.go .
RUN go build -o /bin/echoserver echoserver.go

ENTRYPOINT [ "/bin/echoserver" ]
