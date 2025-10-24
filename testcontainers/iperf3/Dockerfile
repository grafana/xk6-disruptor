FROM debian:trixie@sha256:72547dd722cd005a8c2aa2079af9ca0ee93aad8e589689135feaed60b0a8c08d

RUN apt update && apt install -y iperf3

ENTRYPOINT [ "iperf3" ]
