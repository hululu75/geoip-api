# GeoIP API

A lightweight GeoIP query service built with Go, deployed via Docker.

**Designed to work with the [Traefik Geoblock Plugin](https://github.com/PascalMinder/geoblock)** for geo-based access control in Traefik reverse proxy.

## Features

- Query country code by IP address
- RESTful API interface compatible with Traefik geoblock plugin
- Docker containerized deployment
- Health check endpoint
- Fast and lightweight Go implementation

## Prerequisites

1. Docker and Docker Compose
2. GeoLite2-Country database file

### Download GeoIP Database

Download the free GeoLite2-Country database from MaxMind:

1. Visit https://dev.maxmind.com/geoip/geolite2-free-geolocation-data
2. Sign up for a free account
3. Download the `GeoLite2-Country.mmdb` file
4. Save it to a local path (e.g., `/path/to/GeoLite2-Country.mmdb`)

## Docker Images

Pre-built Docker images are automatically built and published to GitHub Container Registry:

```bash
# Pull the latest image
docker pull ghcr.io/hululu75/geoip-api:latest

# Or use a specific version
docker pull ghcr.io/hululu75/geoip-api:v1.0.0
```

Available image tags:
- `latest` - Latest build from master branch
- `v*.*.*` - Specific version tags
- `master` - Latest master branch build

Supported platforms:
- `linux/amd64`
- `linux/arm64`

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

**Option A: Using pre-built image (recommended)**

Edit `docker-compose.yml` and uncomment the image line:
```yaml
services:
  geoip-api:
    # Comment out the build line
    # build: .
    # Uncomment the image line
    image: ghcr.io/hululu75/geoip-api:latest
```

Then start the service:
```bash
docker-compose up -d
```

**Option B: Build locally**

Keep the default `docker-compose.yml` configuration and run:
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

## Use with Traefik Geoblock Plugin

This GeoIP API is designed to work with the [geoblock Traefik plugin](https://github.com/PascalMinder/geoblock) to block or allow traffic based on geographic location.

**Self-hosted alternative to external APIs:** Instead of relying on external services like `https://get.geojs.io/v1/ip/country/{ip}`, you can use this self-hosted API for better privacy, reliability, and no rate limiting.

### Traefik Configuration Example

Replace the default external API (`https://get.geojs.io/v1/ip/country/{ip}`) with your self-hosted API:

```yaml
# docker-compose.yml for Traefik
services:
  traefik:
    image: traefik:latest
    command:
      - "--experimental.plugins.geoblock.modulename=github.com/PascalMinder/geoblock"
      - "--experimental.plugins.geoblock.version=v0.2.7"
    labels:
      # Use your self-hosted API instead of https://get.geojs.io/v1/ip/country/{ip}
      - "traefik.http.middlewares.geoblock.plugin.geoblock.api=http://geoip-api:8080/{ip}"
      - "traefik.http.middlewares.geoblock.plugin.geoblock.allowedCountries=US,CA,GB"
      - "traefik.http.middlewares.geoblock.plugin.geoblock.logAllowedRequests=true"
      - "traefik.http.middlewares.geoblock.plugin.geoblock.logApiRequests=true"
    networks:
      - geoip

networks:
  geoip:
    external: true
```

### Integration Steps

1. Ensure both services are on the same Docker network
2. Replace the default API URL in your geoblock configuration:
   - **Default:** `api: "https://get.geojs.io/v1/ip/country/{ip}"`
   - **Self-hosted:** `api: "http://geoip-api:8080/{ip}"`
3. Set your allowed or blocked countries using ISO country codes
4. Apply the middleware to your Traefik routes

### Benefits of Self-hosted API

- **Privacy:** IP lookups stay within your infrastructure
- **Reliability:** No dependency on external services
- **No rate limits:** Unlimited queries
- **Performance:** Lower latency for local queries
- **Cost:** Free to use with GeoLite2 database

For more details, see the [geoblock plugin documentation](https://github.com/PascalMinder/geoblock).

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

