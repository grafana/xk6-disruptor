FROM golang:1.21-bookworm as build

WORKDIR /app
COPY . .
RUN go build -o redis-go

FROM debian:bookworm
COPY --from=build /app/redis-go /bin
ENTRYPOINT [ "/bin/redis-go" ]
