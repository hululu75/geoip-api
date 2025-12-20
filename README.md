# GeoIP API

A lightweight GeoIP query service built with Go, deployed via Docker.

## Features

- Query country code by IP address
- RESTful API interface
- Docker containerized deployment
- Health check endpoint

## Prerequisites

1. Docker and Docker Compose
2. GeoLite2-Country database file

### Download GeoIP Database

Download the free GeoLite2-Country database from MaxMind:

1. Visit https://dev.maxmind.com/geoip/geolite2-free-geolocation-data
2. Sign up for a free account
3. Download the `GeoLite2-Country.mmdb` file
4. Save it to a local path (e.g., `/path/to/GeoLite2-Country.mmdb`)

## Quick Start

### 1. Configure Environment Variables

Copy the example configuration file and modify it:

```bash
cp .env.example .env
```

Edit the `.env` file and set the GeoIP database path:

```bash
# Required: Path to GeoIP database on host machine
GEOIP_DB_HOST_PATH=/path/to/your/GeoLite2-Country.mmdb

# Optional: Other configurations
HOST_PORT=8080
CONTAINER_PORT=8080
GEOIP_DB_PATH=/data/GeoLite2-Country.mmdb
```

**Important:** `GEOIP_DB_HOST_PATH` must be set to the actual path of your local GeoLite2-Country.mmdb file, otherwise the container will fail to start!

### 2. Start the Service

```bash
docker-compose up -d
```

### 3. Verify the Service

```bash
# Health check
curl http://localhost:8080/health

# Query IP
curl http://localhost:8080/8.8.8.8
# Returns: US
```

## API Usage

### Query IP Address

```bash
GET /{ip}
```

Examples:

```bash
curl http://localhost:8080/8.8.8.8
# Response: US

curl http://localhost:8080/1.1.1.1
# Response: AU
```

### Health Check

```bash
GET /health
```

Example:

```bash
curl http://localhost:8080/health
# Response: OK
```

## Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `GEOIP_DB_HOST_PATH` | Path to GeoIP database file on host machine | None | **Yes** |
| `HOST_PORT` | Port to expose on host machine | 8080 | No |
| `CONTAINER_PORT` | Port inside container | 8080 | No |
| `GEOIP_DB_PATH` | Database path inside container | /data/GeoLite2-Country.mmdb | No |

### Container Management

```bash
# Start service
docker-compose up -d

# View logs
docker-compose logs -f

# Stop service
docker-compose down

# Restart service
docker-compose restart
```

## Troubleshooting

### Container Fails to Start

If the container fails to start, check:

1. **Database file path is correct**
   ```bash
   # Check GEOIP_DB_HOST_PATH in .env file
   cat .env

   # Verify file exists
   ls -l /path/to/your/GeoLite2-Country.mmdb
   ```

2. **View container logs**
   ```bash
   docker-compose logs geoip-api
   ```

   Common error messages:
   - `Failed to open GeoIP database` - Database file path is incorrect or file does not exist
   - `no such file or directory` - GEOIP_DB_HOST_PATH not set or path is wrong

3. **Permission issues**
   ```bash
   # Ensure database file is readable
   chmod 644 /path/to/your/GeoLite2-Country.mmdb
   ```

## Development

### Local Build

```bash
# Build image
docker build -t geoip-api .

# Run container
docker run -d \
  -p 8080:8080 \
  -v /path/to/GeoLite2-Country.mmdb:/data/GeoLite2-Country.mmdb:ro \
  -e GEOIP_DB_PATH=/data/GeoLite2-Country.mmdb \
  geoip-api
```

### Tech Stack

- Go 1.21
- Alpine Linux 3.19
- geoip2-golang library
- MaxMind GeoLite2 database

## License

This project is licensed under the MIT License.

GeoLite2 database is provided by MaxMind and subject to their terms of use.
