FROM debian:bookworm

RUN apt update && apt install -y tcpdump

ENTRYPOINT ["/bin/tcpdump"]
