FROM alpine:3.15

RUN apk update && apk add iproute2 iptables libc6-compat

WORKDIR /home/xk6-disruptor

COPY build/xk6-disruptor-agent /usr/bin/xk6-disruptor-agent
