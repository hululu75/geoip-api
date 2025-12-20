FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY main.go .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o geoip-api .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/geoip-api .

ENV PORT=8080
ENV GEOIP_DB_PATH=/data/GeoLite2-Country.mmdb

EXPOSE 8080

CMD ["./geoip-api"]
