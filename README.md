# geoip-api

A simple, high-performance GeoIP API written in Go, providing country, city, and region lookup functionalities. It supports automatic MaxMind GeoLite2 database downloads and updates, and can be easily deployed with Docker.

Designed to work with the [Geo Access Control Plugin](https://github.com/hululu75/geo-access-control) and other Traefik middleware plugins (such as Traefik Geoblock Plugin) for geo-based access control in Traefik reverse proxy.

## Features

*   **Fast GeoIP Lookups:** Efficiently retrieves geographical information based on IP addresses.
*   **Multiple Granularity:** Supports country, city, and region lookups (city and region depend on the GeoLite2-City database).
*   **Automatic Database Management:** Downloads and periodically updates MaxMind GeoLite2 databases using a provided license key.
*   **Flexible Output:** Returns data in plain text or JSON format.
*   **Docker Support:** Easy deployment using Docker and Docker Compose.
*   **Health Check Endpoint:** `/health` for monitoring.

## Getting Started

### Prerequisites

*   [Docker](https://docs.docker.com/get-docker/) (for containerized deployment)
*   [Go](https://golang.org/doc/install) (version 1.21 or higher, for local development)
*   **MaxMind GeoLite2 Database License Key:** You need a license key from MaxMind to download the GeoLite2 database. You can obtain one by [signing up for a GeoLite2 Free Account](https://www.maxmind.com/en/geolite2/signup).

## Quick Start

Follow these steps to quickly get the GeoIP API running and make your first request:

1.  **Configure your MaxMind License Key:**
    Copy `.env.example` to `.env` and replace `your_license_key_here` with your actual MaxMind license key.

    ```bash
    cp .env.example .env
    # Edit .env and set MAXMIND_LICENSE_KEY
    ```

2.  **Launch the API with Docker Compose:**
    From the project root, run:

    ```bash
    docker-compose up -d
    ```

    This will build the Docker image (if not already built), download the GeoLite2 database, and start the API in the background.

3.  **Make your first API call:**
    Once the service is running (it might take a minute for the database to download on first run), you can query an IP address.

    ```bash
    curl http://localhost:8080/country/8.8.8.8
    ```

    **Expected Output:**

    ```
    US
    ```

You can now explore other endpoints and configurations detailed below.

### Running with Docker Compose

The easiest way to get started is using `docker-compose`. Ensure your `docker-compose.yml` is configured with your `MAXMIND_LICENSE_KEY`.

1.  **Configure `.env` file:**
    Create a `.env` file in the project root based on `.env.example` and replace `YOUR_MAXMIND_LICENSE_KEY` with your actual key.

    ```bash
    cp .env.example .env
    # Edit .env and set MAXMIND_LICENSE_KEY
    ```

2.  **Start the service:**

    ```bash
    docker-compose up --build -d
    ```

    This will build the Docker image, download the GeoLite2 database, and start the API.

3.  **Check the logs (optional):**

    ```bash
    docker-compose logs -f
    ```

### Running with Docker

1.  **Build the Docker image:**

    ```bash
    docker build -t geoip-api .
    ```

2.  **Run the Docker container:**

    Replace `YOUR_MAXMIND_LICENSE_KEY` with your actual key. You can also mount a volume for `/data` to persist the database.

    ```bash
    docker run -d -p 8080:8080 \
      -e MAXMIND_LICENSE_KEY="YOUR_MAXMIND_LICENSE_KEY" \
      -e GEOIP_DB_PATH="/data/GeoLite2-City.mmdb" \
      -v "$(pwd)/data:/data" \
      --name geoip-api \
      geoip-api
    ```
    *Note:* Using `GeoLite2-City.mmdb` enables city and region lookups. If not specified, it defaults to `GeoLite2-Country.mmdb`.

### Running Locally

1.  **Set environment variables:**

    ```bash
    export MAXMIND_LICENSE_KEY="YOUR_MAXMIND_LICENSE_KEY"
    export GEOIP_DB_PATH="./GeoLite2-City.mmdb" # Or any path you prefer
    ```

2.  **Run the application:**

    ```bash
    go run main.go
    ```

## Configuration (Environment Variables)

The API can be configured using the following environment variables:

| Variable                     | Description                                                                                                                                                                                                                                                                                                                                     | Default                                   |
| :--------------------------- | :---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :---------------------------------------- |
| `MAXMIND_LICENSE_KEY`        | **Required.** Your MaxMind GeoLite2 license key.                                                                                                                                                                                                                                                                                                | `(none)`                                  |
| `PORT`                       | The port on which the API server will listen.                                                                                                                                                                                                                                                                                                   | `8080`                                    |
| `GEOIP_DB_PATH`              | Absolute path to the GeoIP database file (`.mmdb`). This takes precedence over `GEOIP_DB_DIR` and `GEOIP_DB_FILENAME`.                                                                                                                                                                                                                           | `/data/GeoLite2-Country.mmdb`             |
| `GEOIP_DB_DIR`               | Directory where the GeoIP database file will be stored. Used in conjunction with `GEOIP_DB_FILENAME`.                                                                                                                                                                                                                                           | `(none)`                                  |
| `GEOIP_DB_FILENAME`          | Filename of the GeoIP database. If `GEOIP_DB_DIR` is set and this is not, defaults to `GeoLite2-Country.mmdb`. Specify `GeoLite2-City.mmdb` for city/region data.                                                                                                                                                                                | `GeoLite2-Country.mmdb`                   |
| `DB_UPDATE_INTERVAL_HOURS`   | Interval in hours for periodically checking and updating the GeoIP database. Set to `0` to disable automatic updates.                                                                                                                                                                                                                             | `720` (30 days)                           |
| `FORCE_DB_UPDATE`            | If set to `true`, forces a database download/update on startup, regardless of its age.                                                                                                                                                                                                                                                          | `false`                                   |
| `LOG_LEVEL`                  | Sets the logging level. Can be `ERROR`, `INFO`, or `DEBUG`.                                                                                                                                                                                                                                                                                     | `INFO`                                    |

## API Endpoints

All endpoints support an optional `?format=json` query parameter for JSON output. If omitted, plain text is returned.

### `GET /`

Provides general information about the API and usage examples.

### `GET /country/{ip}`

Returns the ISO 3166-1 alpha-2 country code for the given IP address.

**Example (Plain Text):**

```bash
curl http://localhost:8080/country/8.8.8.8
# Output: US
```

**Example (JSON):**

```bash
curl http://localhost:8080/country/8.8.8.8?format=json
# Output: {"ip":"8.8.8.8","country":"US"}
```

### `GET /city/{ip}`

Returns the country code, city name, and region code for the given IP address.
*Note: This endpoint requires the `GeoLite2-City.mmdb` database.*

**Example (Plain Text):**

```bash
curl http://localhost:8080/city/8.8.8.8
# Output: US|Mountain View|CA
```

**Example (JSON):**

```bash
curl http://localhost:8080/city/8.8.8.8?format=json
# Output: {"ip":"8.8.8.8","country":"US","city":"Mountain View","region":"CA"}
```

### `GET /region/{ip}`

Returns the country code and region code for the given IP address.
*Note: This endpoint requires the `GeoLite2-City.mmdb` database.*

**Example (Plain Text):**

```bash
curl http://localhost:8080/region/8.8.8.8
# Output: US|CA
```

**Example (JSON):**

```bash
curl http://localhost:8080/region/8.8.8.8?format=json
# Output: {"ip":"8.8.8.8","country":"US","region":"CA"}
```

### `GET /health`

Returns `OK` if the API is running.

**Example:**

```bash
curl http://localhost:8080/health
# Output: OK
```

## Integration with Traefik Plugins

This GeoIP API is designed to work seamlessly with Traefik middleware plugins for geo-based access control. It provides the geographic data backend that these plugins use to enforce access rules.

### Geo Access Control Plugin

This API is primarily designed to work with [geo-access-control](https://github.com/hululu75/geo-access-control), a powerful Traefik middleware plugin that provides comprehensive geographic access control based on country, region, city, and IP addresses.

**Key Features when used together:**

*   **Unified Access Rules:** Define allow/deny rules for countries, regions, cities, and specific IPs in a single configuration
*   **Hierarchical Rule Logic:** Apply "most specific rule wins" with IP rules taking precedence over geographic rules
*   **Flexible Response Handling:** Customize HTTP status codes, response messages, or redirect blocked requests
*   **LRU Caching:** The plugin caches GeoIP lookups for improved performance
*   **Path Exclusions:** Exclude specific paths from geographic checks using regex patterns
*   **Private IP Handling:** Optionally allow local/private IPs to bypass checks

**Example Configuration:**

```yaml
# docker-compose.yml
services:
  geoip-api:
    image: geoip-api
    environment:
      - MAXMIND_LICENSE_KEY=your_license_key
      - GEOIP_DB_PATH=/data/GeoLite2-City.mmdb
    ports:
      - "8080:8080"
    volumes:
      - ./data:/data

# Traefik dynamic configuration
http:
  middlewares:
    geo-access-control:
      plugin:
        geo-access-control:
          geoIPApiUrl: "http://geoip-api:8080"
          accessRules:
            allowCountries:
              - US
              - CA
            denyRegions:
              - "CN|HK"
            allowIPs:
              - "192.168.1.0/24"
          onBlock:
            statusCode: 403
            message: "Access denied from your location"
```

### Compatibility with Other Plugins

This API is also compatible with other Traefik GeoIP-based plugins, including:

*   **Traefik Geoblock Plugin:** Works with any plugin that expects standard GeoIP API endpoints
*   **Custom Middleware:** Can be integrated into custom Traefik middleware that requires geographic data

The API's flexible output format (plain text and JSON) and standard endpoint structure (`/country/{ip}`, `/city/{ip}`, `/region/{ip}`) ensure broad compatibility with various geo-blocking and access control solutions.

**API Output Formats:**

*   **Plain Text:** Simple, pipe-delimited format (e.g., `US|CA` for region queries)
*   **JSON:** Structured data format for easier parsing in custom middleware

This makes it easy to integrate with existing tools or build your own geographic access control solutions.