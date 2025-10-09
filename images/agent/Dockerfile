FROM alpine:3.20@sha256:765942a4039992336de8dd5db680586e1a206607dd06170ff0a37267a9e01958

ARG TARGETARCH

RUN apk update && apk add iproute2 iptables libc6-compat

WORKDIR /home/xk6-disruptor

COPY build/xk6-disruptor-agent-linux-${TARGETARCH} /usr/bin/xk6-disruptor-agent
