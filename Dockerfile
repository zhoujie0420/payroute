FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY . .
RUN go build -o /payroute ./cmd/server

FROM alpine:latest
WORKDIR /app
COPY --from=builder /payroute /app/payroute
COPY web/static /app/web/static
EXPOSE 8080
ENV PORT=8080
CMD ["/app/payroute"]
