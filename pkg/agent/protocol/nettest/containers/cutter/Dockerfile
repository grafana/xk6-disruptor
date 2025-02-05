FROM debian:bookworm

RUN apt update && apt install -y curl zstd

# Upstream for cutter website is unavailable at the time of writing, the arch linux package is the most stable
# source I was able to find.
RUN curl -vL https://archive.archlinux.org/packages/c/cutter/cutter-1.04-3-x86_64.pkg.tar.zst \
    | tar --zstd -xvC /

