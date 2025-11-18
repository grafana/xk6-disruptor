FROM debian:trixie@sha256:8f6a88feef3ed01a300dafb87f208977f39dccda1fd120e878129463f7fa3b8f

RUN apt update && apt install -y iperf3

ENTRYPOINT [ "iperf3" ]
