# geoip-api

A simple, high-performance GeoIP API written in Go, providing country, city, and region lookup functionalities. It supports automatic MaxMind GeoLite2 database downloads and updates, and can be easily deployed with Docker.

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