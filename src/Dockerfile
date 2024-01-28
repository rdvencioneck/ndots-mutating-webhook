FROM golang:1.21.6 as builder
COPY . /src
WORKDIR /src
RUN mkdir -p ./bin
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/ndots-injector .

FROM ubuntu:22.04
COPY --from=builder /src/bin/ndots-injector .
CMD ["./ndots-injector"]
