FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o geoip-api .

FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/geoip-api .

ENV PORT=8080
ENV GEOIP_DB_PATH=/data/GeoLite2-Country.mmdb
ENV TZ=UTC

EXPOSE 8080

CMD ["./geoip-api"]
