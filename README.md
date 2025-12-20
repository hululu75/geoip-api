# GeoIP API

A lightweight GeoIP query service built with Go, deployed via Docker.

**Designed to work with the [Traefik Geoblock Plugin](https://github.com/PascalMinder/geoblock)** for geo-based access control in Traefik reverse proxy.

## Features

- Query country code by IP address
- RESTful API interface compatible with Traefik geoblock plugin
- Docker containerized deployment
- Health check endpoint
- Fast and lightweight Go implementation

## Automatic GeoIP Database Download and Update

This service can automatically download and update the GeoLite2-Country database from MaxMind upon startup, eliminating the need for manual downloads and storage.

### How it Works:

1.  **Configuration**: Provide your `MAXMIND_LICENSE_KEY` environment variable. You can also specify the `GEOIP_DB_DIR`, `GEOIP_DB_FILENAME`, `FORCE_DB_UPDATE`, and `DB_UPDATE_INTERVAL_HOURS`.
2.  **Initial Download**: If the database file is not found at the configured path, the service will attempt to download the latest `GeoLite2-Country.mmdb` using your `MAXMIND_LICENSE_KEY`.
3.  **Automatic Updates**: On subsequent startups, the service checks the age of the local database file. If it's older than `DB_UPDATE_INTERVAL_HOURS` (default: 30 days), a new download is initiated.
4.  **Forced Updates**: Setting `FORCE_DB_UPDATE` to `true` will always trigger a download and update.
5.  **Robust Verification**: Before replacing the existing database, the newly downloaded file undergoes a two-step verification process:
    *   It is checked for validity by attempting to open it with the `geoip2-golang` library.
    *   A sample IP lookup (e.g., 8.8.8.8) is performed to ensure its functionality and content accuracy.
    Only if both checks pass will the old database be atomically replaced.
6.  **Zero Downtime Updates**: The application continues to use the currently loaded database while the new one is being downloaded and verified in the background. The updated database is loaded only upon the next application restart.


## Prerequisites

1. Docker and Docker Compose
2. **MaxMind GeoLite2 License Key (for automatic download)**: If you plan to use the automatic download feature, you'll need a free MaxMind GeoLite2 License Key.
   - Visit https://www.maxmind.com/en/geolite2/signup to sign up for a free account.
   - Once registered, generate a license key from your account portal.

### GeoIP Database File (Manual or Automatic)

You can either manually provide the `GeoLite2-Country.mmdb` file or let the service download it automatically.

**Option A: Automatic Download (Recommended)**
   - Provide your `MAXMIND_LICENSE_KEY` as an environment variable. The service will download and update the database on startup.

**Option B: Manual Download**
   - Download the free GeoLite2-Country database from MaxMind:
     1. Visit https://dev.maxmind.com/geoip/geolite2-free-geolocation-data
     2. Download the `GeoLite2-Country.mmdb` file.
     3. Save it to a local path (e.g., `/path/to/GeoLite2-Country.mmdb`) and configure `GEOIP_DB_PATH` or `GEOIP_DB_DIR` accordingly.

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
# Optional: Directory containing GeoIP database on host machine
# Default: ./data (project root/data directory)
GEOIP_DB_DIR=/path/to/geoip-data

# Optional: GeoIP database filename
# Default: GeoLite2-Country.mmdb
GEOIP_DB_FILENAME=GeoLite2-Country.mmdb

# Optional: Other configurations
HOST_PORT=8080
CONTAINER_PORT=8080
```

**Note:**
- If `GEOIP_DB_DIR` is not set, the service will look for the database in `./data` directory (relative to docker-compose.yml)
- If `GEOIP_DB_FILENAME` is not set, it defaults to `GeoLite2-Country.mmdb`
- Make sure the database file exists in the specified directory

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
curl http://localhost:8080/country/8.8.8.8
# Returns: US
```

## API Usage

### Query IP Address

```bash
GET /country/{ip}
GET /country/{ip}?format=json
```

**Parameters:**
- `format` (optional): Response format. Values: `json` (default: plain text)

**Examples:**

**Text format (default):**
```bash
curl http://localhost:8080/country/8.8.8.8
# Response: US

curl http://localhost:8080/country/1.1.1.1
# Response: AU
```

**JSON format:**
```bash
curl http://localhost:8080/country/8.8.8.8?format=json
# Response: {"ip":"8.8.8.8","country":"US"}

curl http://localhost:8080/country/1.1.1.1?format=json
# Response: {"ip":"1.1.1.1","country":"AU"}
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
      - "traefik.http.middlewares.geoblock.plugin.geoblock.api=http://geoip-api:8080/country/{ip}"
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
   - **Self-hosted:** `api: "http://geoip-api:8080/country/{ip}"`
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

| Variable | Description | Default | Required (for auto-download) |
|----------|-------------|---------|------------------------------|
| `MAXMIND_LICENSE_KEY` | Your MaxMind GeoLite2 License Key. Required for automatic download and updates. | None | Yes |
| `GEOIP_DB_PATH` | Full absolute path to the GeoIP database file (e.g., `/data/GeoLite2-Country.mmdb`). Overrides `GEOIP_DB_DIR` and `GEOIP_DB_FILENAME`. | None | No |
| `GEOIP_DB_DIR` | Directory where the GeoIP database file will be stored or looked for. Used in conjunction with `GEOIP_DB_FILENAME`. | `./data` | No |
| `GEOIP_DB_FILENAME` | Filename of the GeoIP database (e.g., `my-custom-geo.mmdb`). Used in conjunction with `GEOIP_DB_DIR`. | `GeoLite2-Country.mmdb` | No |
| `FORCE_DB_UPDATE` | Set to `true` to force a database download and update on startup, regardless of age. | `false` | No |
| `DB_UPDATE_INTERVAL_HOURS` | Interval in hours after which the database will be considered outdated and trigger an automatic update on startup. | `720` (30 days) | No |
| `HOST_PORT` | Port to expose on host machine | `8080` | No |
| `CONTAINER_PORT` | Port inside container | `8080` | No |

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

1. **MaxMind License Key (for auto-download)**
   - If using automatic download, ensure `MAXMIND_LICENSE_KEY` is correctly set in your environment or `.env` file. Without it, auto-download will fail.

2. **Database directory and file are correct**
   ```bash
   # Check GEOIP_DB_PATH, GEOIP_DB_DIR, and GEOIP_DB_FILENAME in .env file
   cat .env

   # Verify database file exists in the directory
   # (replace with your actual directory and filename)
   ls -l /path/to/geoip-data/GeoLite2-Country.mmdb
   # Or if using default ./data directory:
   ls -l ./data/GeoLite2-Country.mmdb
   ```

3. **View container logs**
   ```bash
   docker-compose logs geoip-api
   ```

   Common error messages:
   - `MAXMIND_LICENSE_KEY not set` - Self-explanatory, set the key.
   - `Failed to open GeoIP database` - Database file does not exist in the mounted directory or is corrupted.
   - `no such file or directory` - `GEOIP_DB_DIR` not set correctly or `GeoLite2-Country.mmdb` missing/incorrect filename.

4. **Permission issues**
   ```bash
   # Ensure database file is readable
   chmod 644 /path/to/geoip-data/GeoLite2-Country.mmdb
   ```

## Development

### Local Build

```bash
# Build image
docker build -t geoip-api .

# Run container
docker run -d \
  -p 8080:8080 \
  -v /path/to/geoip-data:/data:ro \
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

