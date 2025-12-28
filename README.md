# GeoIP API

A lightweight GeoIP query service built with Go, deployed via Docker.

**Designed to work with the [Traefik Geoblock Plugin](https://github.com/PascalMinder/geoblock)** for geo-based access control in Traefik reverse proxy.

## Features

- Query country code by IP address
- RESTful API interface compatible with Traefik geoblock plugin
- Docker containerized deployment
- Health check endpoint
- Fast and lightweight Go implementation
- **Automatic periodic database updates** - Background service checks and updates database without restart
- **Configurable log levels** - DEBUG, INFO, ERROR modes for production and troubleshooting

## Automatic GeoIP Database Download and Update

This service can automatically download and update the GeoLite2-Country database from MaxMind, eliminating the need for manual downloads and storage.

### How it Works:

1.  **Configuration**: Provide your `MAXMIND_LICENSE_KEY` environment variable. You can also specify the `GEOIP_DB_DIR`, `GEOIP_DB_FILENAME`, `FORCE_DB_UPDATE`, and `DB_UPDATE_INTERVAL_HOURS`.
2.  **Initial Download**: If the database file is not found at the configured path, the service will attempt to download the latest `GeoLite2-Country.mmdb` using your `MAXMIND_LICENSE_KEY`.
3.  **Periodic Background Updates**: After startup, a background service automatically checks the database age every `DB_UPDATE_INTERVAL_HOURS` (default: 30 days). If the database is outdated, it will be automatically downloaded and reloaded **without requiring a service restart**.
4.  **Forced Updates**: Setting `FORCE_DB_UPDATE` to `true` will always trigger a download and update on startup.
5.  **Robust Verification**: Before replacing the existing database, the newly downloaded file undergoes a two-step verification process:
    *   It is checked for validity by attempting to open it with the `geoip2-golang` library.
    *   A sample IP lookup (e.g., 8.8.8.8) is performed to ensure its functionality and content accuracy.
    Only if both checks pass will the old database be atomically replaced.
6.  **Zero Downtime Updates**: The service continues to serve requests using the current database while the new one is being downloaded and verified. Database updates are performed atomically with hot-reload - no restart required!


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

Edit the `.env` file and configure as needed:

```bash
# MaxMind License Key (required for automatic database download/updates)
MAXMIND_LICENSE_KEY=your_license_key_here

# Optional: Directory containing GeoIP database on host machine
# Default: ./data (project root/data directory)
GEOIP_DB_DIR=/path/to/geoip-data

# Optional: GeoIP database filename
# Default: GeoLite2-Country.mmdb
GEOIP_DB_FILENAME=GeoLite2-Country.mmdb

# Optional: Database update interval in hours (must be integer ≥ 1)
# Default: 720 (30 days)
# Examples: 24 (daily), 168 (weekly), 720 (monthly)
DB_UPDATE_INTERVAL_HOURS=24

# Optional: Log level (ERROR, INFO, DEBUG)
# Default: INFO
LOG_LEVEL=INFO

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

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `MAXMIND_LICENSE_KEY` | Your MaxMind GeoLite2 License Key. Required for automatic download and updates. | None | Yes (for auto-download) |
| `GEOIP_DB_PATH` | Full absolute path to the GeoIP database file (e.g., `/data/GeoLite2-Country.mmdb`). Overrides `GEOIP_DB_DIR` and `GEOIP_DB_FILENAME`. | None | No |
| `GEOIP_DB_DIR` | Directory where the GeoIP database file will be stored or looked for. Used in conjunction with `GEOIP_DB_FILENAME`. | `./data` | No |
| `GEOIP_DB_FILENAME` | Filename of the GeoIP database (e.g., `my-custom-geo.mmdb`). Used in conjunction with `GEOIP_DB_DIR`. | `GeoLite2-Country.mmdb` | No |
| `FORCE_DB_UPDATE` | Set to `true` to force a database download and update on startup, regardless of age. | `false` | No |
| `DB_UPDATE_INTERVAL_HOURS` | Interval in hours for periodic database age checks. The background service checks every N hours and updates if database is older than N hours. **Must be an integer ≥ 1** (e.g., `24` for daily checks, `168` for weekly). Decimals like `0.5` are not supported. | `720` (30 days) | No |
| `LOG_LEVEL` | Logging verbosity level. Options: `ERROR` (errors only), `INFO` (normal operation, recommended for production), `DEBUG` (detailed debugging info including all IP lookups). | `INFO` | No |
| `HOST_PORT` | Port to expose on host machine | `8080` | No |
| `CONTAINER_PORT` | Port inside container | `8080` | No |

### Logging Configuration

The service supports three log levels via the `LOG_LEVEL` environment variable:

#### **ERROR** - Production (Minimal Logging)
Only logs critical errors and failures. Recommended for production environments where you want minimal log output.

```yaml
environment:
  - LOG_LEVEL=ERROR
```

**Example output:**
```
[ERROR] Failed to update database: connection timeout
[ERROR] Failed to get file info for /data/GeoLite2-Country.mmdb: permission denied
```

#### **INFO** - Production (Recommended)
Logs important operational events including startup, database updates, and warnings. **Default and recommended for most users.**

```yaml
environment:
  - LOG_LEVEL=INFO
```

**Example output:**
```
[INFO] GeoIP database at /data/GeoLite2-Country.mmdb is up to date.
[INFO] Started periodic database updater (interval: 24 hours)
[INFO] GeoIP API listening on port 8080
[INFO] Database is older than 24 hours, starting update...
[INFO] Database updated and reloaded successfully
```

#### **DEBUG** - Development & Troubleshooting
Logs detailed information including configuration values, database age checks, download progress, and **every IP lookup request**. Use for debugging issues.

```yaml
environment:
  - LOG_LEVEL=DEBUG
```

**Example output:**
```
[DEBUG] Log level set to: DEBUG
[DEBUG] Configuration - DB Path: /data/GeoLite2-Country.mmdb, Update Interval: 24 hours, Force Update: false
[DEBUG] Database file last modified: 2025-12-27T10:30:00Z (age: 25.5 hours)
[INFO] Started periodic database updater (interval: 24 hours)
[INFO] GeoIP API listening on port 8080
[DEBUG] IP lookup: 8.8.8.8 -> US
[DEBUG] IP lookup: 1.1.1.1 -> AU
[DEBUG] Periodic check triggered - checking if database needs to be updated...
[DEBUG] Database age: 25.5 hours (threshold: 24 hours)
[INFO] Database is older than 24 hours, starting update...
[DEBUG] Starting database download from MaxMind
[DEBUG] Download successful, extracting archive...
[DEBUG] Verifying downloaded database: /tmp/geoipdb123/GeoLite2-Country.mmdb
[DEBUG] Verification successful: Test IP 8.8.8.8 correctly identified as US.
[DEBUG] Moving verified database from /tmp/geoipdb123/GeoLite2-Country.mmdb to /data/GeoLite2-Country.mmdb
[DEBUG] Database file successfully updated at /data/GeoLite2-Country.mmdb
[INFO] Database updated and reloaded successfully
```

**⚠️ Note:** DEBUG mode logs every IP lookup request, which can generate significant log volume in high-traffic environments. Use only for troubleshooting.

### Container Management

```bash
# Start service
docker-compose up -d

# View logs (real-time)
docker-compose logs -f

# View logs with timestamps
docker-compose logs -f -t geoip-api

# View last 100 log lines
docker-compose logs --tail=100 geoip-api

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
   # Use DEBUG mode for detailed troubleshooting
   LOG_LEVEL=DEBUG docker-compose up

   # Or view existing logs
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

### Database Not Updating Automatically

If the database is not updating as expected:

1. **Check update interval configuration**
   - Verify `DB_UPDATE_INTERVAL_HOURS` is set correctly (must be an integer ≥ 1)
   - Invalid values like `0.5` will fall back to the default (720 hours)

2. **Enable DEBUG logging to monitor update checks**
   ```bash
   # Add to your .env or docker-compose.yml
   LOG_LEVEL=DEBUG

   # Restart and watch logs
   docker-compose restart
   docker-compose logs -f geoip-api
   ```

3. **Look for update check messages in DEBUG mode**
   ```
   [DEBUG] Periodic check triggered - checking if database needs to be updated...
   [DEBUG] Database age: 25.5 hours (threshold: 24 hours)
   [INFO] Database is older than 24 hours, starting update...
   ```

4. **Force an immediate update**
   ```bash
   # Add to .env temporarily
   FORCE_DB_UPDATE=true

   # Restart service
   docker-compose restart

   # Remove FORCE_DB_UPDATE after successful update
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

