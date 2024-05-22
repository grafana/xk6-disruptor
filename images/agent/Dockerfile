FROM alpine:3.20

ARG TARGETARCH

RUN apk update && apk add iproute2 iptables libc6-compat

WORKDIR /home/xk6-disruptor

COPY build/xk6-disruptor-agent-linux-${TARGETARCH} /usr/bin/xk6-disruptor-agent
