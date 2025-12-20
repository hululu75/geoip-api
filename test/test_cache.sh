#!/bin/bash

# GeoIP API Cache Performance Test Script
# This script tests the API with cache enabled and disabled

set -e

# Configuration
PORT=${PORT:-12345}
BASE_URL="http://localhost:${PORT}"
GEOIP_DB_PATH=${GEOIP_DB_PATH:-/root/docker/geoip/GeoLite2-Country.mmdb}
CACHE_MAX_SIZE=${CACHE_MAX_SIZE:-1000}

# Test IPs - mix of different IPs and repeated IPs
TEST_IPS=(
    "8.8.8.8"
    "1.1.1.1"
    "8.8.8.8"
    "114.114.114.114"
    "8.8.8.8"
    "1.1.1.1"
    "9.9.9.9"
    "8.8.8.8"
    "1.1.1.1"
    "8.8.8.8"
)

# Number of iterations for performance test
ITERATIONS=100

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    if [ ! -z "$APP_PID" ]; then
        kill $APP_PID 2>/dev/null || true
        wait $APP_PID 2>/dev/null || true
    fi
}

trap cleanup EXIT

# Function to start the application
start_app() {
    local cache_enabled=$1
    local cache_size=$2

    echo -e "${BLUE}Starting application with CACHE_ENABLED=${cache_enabled}, CACHE_MAX_SIZE=${cache_size}...${NC}"

    export GEOIP_DB_PATH=$GEOIP_DB_PATH
    export CACHE_ENABLED=$cache_enabled
    export CACHE_MAX_SIZE=$cache_size
    export PORT=$PORT

    go run main.go > /tmp/geoip-test.log 2>&1 &
    APP_PID=$!

    # Wait for the application to start
    echo -n "Waiting for application to start..."
    for i in {1..10}; do
        if curl -s "${BASE_URL}/health" > /dev/null 2>&1; then
            echo -e " ${GREEN}OK${NC}"
            return 0
        fi
        sleep 1
        echo -n "."
    done

    echo -e " ${RED}FAILED${NC}"
    echo "Application logs:"
    cat /tmp/geoip-test.log
    exit 1
}

# Function to stop the application
stop_app() {
    if [ ! -z "$APP_PID" ]; then
        echo -e "${BLUE}Stopping application (PID: $APP_PID)...${NC}"
        kill $APP_PID 2>/dev/null || true
        wait $APP_PID 2>/dev/null || true
        APP_PID=""
        sleep 2
    fi
}

# Function to test basic functionality
test_basic_functionality() {
    echo -e "\n${BLUE}=== Testing Basic Functionality ===${NC}"

    # Test health endpoint
    echo -n "Health check: "
    if curl -s "${BASE_URL}/health" | grep -q "OK"; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL${NC}"
        return 1
    fi

    # Test text response
    echo -n "Text response (8.8.8.8): "
    RESPONSE=$(curl -s "${BASE_URL}/country/8.8.8.8")
    if [ "$RESPONSE" == "US" ]; then
        echo -e "${GREEN}PASS${NC} (Got: $RESPONSE)"
    else
        echo -e "${RED}FAIL${NC} (Got: $RESPONSE, Expected: US)"
        return 1
    fi

    # Test JSON response
    echo -n "JSON response (8.8.8.8): "
    RESPONSE=$(curl -s "${BASE_URL}/country/8.8.8.8?format=json")
    if echo "$RESPONSE" | grep -q '"country":"US"'; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL${NC} (Got: $RESPONSE)"
        return 1
    fi

    # Test stats endpoint
    echo -n "Stats endpoint: "
    RESPONSE=$(curl -s "${BASE_URL}/stats")
    if echo "$RESPONSE" | grep -q "cache_enabled"; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL${NC}"
        return 1
    fi
}

# Function to run performance test
run_performance_test() {
    local test_name=$1

    # Output to stderr so it doesn't interfere with return value
    echo -e "\n${BLUE}=== Performance Test: $test_name ===${NC}" >&2

    # Clear previous stats by making some requests
    curl -s "${BASE_URL}/health" > /dev/null

    # Warm up
    echo "Warming up (10 requests)..." >&2
    for ip in "${TEST_IPS[@]}"; do
        curl -s "${BASE_URL}/country/$ip" > /dev/null
    done

    # Performance test
    echo "Running performance test ($ITERATIONS iterations)..." >&2
    START_TIME=$(date +%s.%N)

    for i in $(seq 1 $ITERATIONS); do
        for ip in "${TEST_IPS[@]}"; do
            curl -s "${BASE_URL}/country/$ip" > /dev/null
        done
    done

    END_TIME=$(date +%s.%N)
    DURATION=$(awk "BEGIN {print $END_TIME - $START_TIME}")
    TOTAL_REQUESTS=$((ITERATIONS * ${#TEST_IPS[@]}))
    RPS=$(awk "BEGIN {printf \"%.2f\", $TOTAL_REQUESTS / $DURATION}")
    AVG_TIME=$(awk "BEGIN {printf \"%.4f\", $DURATION / $TOTAL_REQUESTS * 1000}")

    # Get stats
    STATS=$(curl -s "${BASE_URL}/stats")

    echo -e "\n${GREEN}Results:${NC}" >&2
    echo "  Total requests: $TOTAL_REQUESTS" >&2
    echo "  Duration: ${DURATION}s" >&2
    echo "  Requests/sec: ${RPS}" >&2
    echo "  Avg response time: ${AVG_TIME}ms" >&2

    echo -e "\n${GREEN}Cache Statistics:${NC}" >&2
    echo "$STATS" | python3 -m json.tool >&2

    # Return values for comparison (stdout only, no colors)
    echo "$RPS|$AVG_TIME|$STATS"
}

# Main test execution
main() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}GeoIP API Cache Performance Test${NC}"
    echo -e "${BLUE}========================================${NC}"

    # Test 1: Cache Enabled
    echo -e "\n${YELLOW}>>> Test 1: Cache ENABLED <<<${NC}"
    start_app "true" "$CACHE_MAX_SIZE"
    test_basic_functionality
    RESULT_CACHE_ON=$(run_performance_test "Cache Enabled")
    stop_app

    # Test 2: Cache Disabled
    echo -e "\n${YELLOW}>>> Test 2: Cache DISABLED <<<${NC}"
    start_app "false" "0"
    test_basic_functionality
    RESULT_CACHE_OFF=$(run_performance_test "Cache Disabled")
    stop_app

    # Comparison
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}Performance Comparison${NC}"
    echo -e "${BLUE}========================================${NC}"

    RPS_ON=$(echo "$RESULT_CACHE_ON" | cut -d'|' -f1)
    AVG_ON=$(echo "$RESULT_CACHE_ON" | cut -d'|' -f2)

    RPS_OFF=$(echo "$RESULT_CACHE_OFF" | cut -d'|' -f1)
    AVG_OFF=$(echo "$RESULT_CACHE_OFF" | cut -d'|' -f2)

    SPEEDUP=$(awk "BEGIN {printf \"%.2f\", $RPS_ON / $RPS_OFF}")
    TIME_IMPROVE=$(awk "BEGIN {printf \"%.2f\", ($AVG_OFF - $AVG_ON) / $AVG_OFF * 100}")

    echo -e "${GREEN}Cache ON:${NC}"
    echo "  Requests/sec: $RPS_ON"
    echo "  Avg response: ${AVG_ON}ms"

    echo -e "\n${RED}Cache OFF:${NC}"
    echo "  Requests/sec: $RPS_OFF"
    echo "  Avg response: ${AVG_OFF}ms"

    echo -e "\n${YELLOW}Improvement:${NC}"
    echo "  Speedup: ${SPEEDUP}x faster"
    echo "  Response time improvement: ${TIME_IMPROVE}%"

    if (( $(awk "BEGIN {print ($SPEEDUP > 1.5)}") )); then
        echo -e "\n${GREEN}✓ Cache provides significant performance improvement!${NC}"
    elif (( $(awk "BEGIN {print ($SPEEDUP > 1.0)}") )); then
        echo -e "\n${YELLOW}✓ Cache provides moderate performance improvement.${NC}"
    else
        echo -e "\n${RED}✗ Cache does not provide expected improvement.${NC}"
    fi

    echo -e "\n${BLUE}Test completed successfully!${NC}"
}

# Run main function
main
