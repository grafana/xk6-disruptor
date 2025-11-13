FROM debian:trixie@sha256:01a723bf5bfb21b9dda0c9a33e0538106e4d02cce8f557e118dd61259553d598

RUN apt update && apt install -y iperf3

ENTRYPOINT [ "iperf3" ]
