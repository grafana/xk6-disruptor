FROM debian:bookworm@sha256:93492d1405a072c9d3b89110490e330e0b7eb37754cafc9520908b7fe6873f5f

RUN apt update && apt install -y iperf3

ENTRYPOINT [ "iperf3" ]
