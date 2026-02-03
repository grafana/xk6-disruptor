FROM debian:trixie@sha256:2c91e484d93f0830a7e05a2b9d92a7b102be7cab562198b984a84fdbc7806d91

RUN apt update && apt install -y iperf3

ENTRYPOINT [ "iperf3" ]
