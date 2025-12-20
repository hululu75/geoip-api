#!/bin/bash

# Quick Cache Test Script - for already running application
# Usage: ./test_cache_quick.sh [port]

PORT=${1:-12345}
BASE_URL="http://localhost:${PORT}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}GeoIP API Quick Cache Test${NC}"
echo -e "${BLUE}Port: $PORT${NC}"
echo -e "${BLUE}========================================${NC}"

# Check if application is running
echo -n "Checking if application is running... "
if ! curl -s "${BASE_URL}/health" > /dev/null 2>&1; then
    echo -e "${RED}FAILED${NC}"
    echo "Application is not running on port $PORT"
    exit 1
fi
echo -e "${GREEN}OK${NC}"

# Get initial stats
echo -e "\n${BLUE}Initial Cache Statistics:${NC}"
curl -s "${BASE_URL}/stats" | python3 -m json.tool

# Test 1: Single IP repeated requests
echo -e "\n${BLUE}=== Test 1: Repeated Requests (Cache Hit Test) ===${NC}"
echo "Sending 10 requests to 8.8.8.8..."
for i in {1..10}; do
    curl -s "${BASE_URL}/country/8.8.8.8" > /dev/null
done

echo -e "${GREEN}Statistics after 10 repeated requests:${NC}"
STATS=$(curl -s "${BASE_URL}/stats")
echo "$STATS" | python3 -m json.tool

HITS=$(echo "$STATS" | python3 -c "import sys, json; print(json.load(sys.stdin)['cache_hits'])")
echo -e "${YELLOW}Expected: 9 cache hits (first miss, then 9 hits)${NC}"
echo -e "${GREEN}Actual cache hits: $HITS${NC}"

# Test 2: Multiple different IPs
echo -e "\n${BLUE}=== Test 2: Different IPs (Cache Miss Test) ===${NC}"
echo "Testing 5 different IPs..."
for ip in 1.1.1.1 9.9.9.9 114.114.114.114 8.8.4.4 208.67.222.222; do
    RESULT=$(curl -s "${BASE_URL}/country/$ip")
    echo "  $ip → $RESULT"
done

echo -e "${GREEN}Statistics after different IPs:${NC}"
curl -s "${BASE_URL}/stats" | python3 -m json.tool

# Test 3: Mixed pattern (realistic usage)
echo -e "\n${BLUE}=== Test 3: Mixed Pattern Test ===${NC}"
echo "Simulating realistic usage (repeated + new IPs)..."

# Reset by getting current stats
BEFORE_STATS=$(curl -s "${BASE_URL}/stats")
BEFORE_HITS=$(echo "$BEFORE_STATS" | python3 -c "import sys, json; print(json.load(sys.stdin)['cache_hits'])")
BEFORE_MISSES=$(echo "$BEFORE_STATS" | python3 -c "import sys, json; print(json.load(sys.stdin)['cache_misses'])")

# Send mixed requests
for i in {1..20}; do
    if [ $((i % 3)) -eq 0 ]; then
        # New IP (cache miss)
        curl -s "${BASE_URL}/country/1.2.3.$i" > /dev/null
    else
        # Repeated IP (cache hit)
        curl -s "${BASE_URL}/country/8.8.8.8" > /dev/null
    fi
done

AFTER_STATS=$(curl -s "${BASE_URL}/stats")
AFTER_HITS=$(echo "$AFTER_STATS" | python3 -c "import sys, json; print(json.load(sys.stdin)['cache_hits'])")
AFTER_MISSES=$(echo "$AFTER_STATS" | python3 -c "import sys, json; print(json.load(sys.stdin)['cache_misses'])")

NEW_HITS=$((AFTER_HITS - BEFORE_HITS))
NEW_MISSES=$((AFTER_MISSES - BEFORE_MISSES))

echo -e "${GREEN}Results:${NC}"
echo "  New cache hits: $NEW_HITS"
echo "  New cache misses: $NEW_MISSES"
if [ $((NEW_HITS + NEW_MISSES)) -gt 0 ]; then
    HIT_RATE_TEST=$(awk "BEGIN {printf \"%.2f\", $NEW_HITS * 100 / ($NEW_HITS + $NEW_MISSES)}")
    echo "  Hit rate for this test: ${HIT_RATE_TEST}%"
else
    echo "  Hit rate for this test: N/A"
fi

echo -e "\n${GREEN}Final Statistics:${NC}"
curl -s "${BASE_URL}/stats" | python3 -m json.tool

# Test 4: Performance comparison
echo -e "\n${BLUE}=== Test 4: Performance Test ===${NC}"
echo "Testing response time (100 requests)..."

START=$(date +%s.%N)
for i in {1..100}; do
    curl -s "${BASE_URL}/country/8.8.8.8" > /dev/null
done
END=$(date +%s.%N)

DURATION=$(awk "BEGIN {print $END - $START}")
AVG_TIME=$(awk "BEGIN {printf \"%.4f\", $DURATION / 100 * 1000}")
RPS=$(awk "BEGIN {printf \"%.2f\", 100 / $DURATION}")

echo -e "${GREEN}Performance Results:${NC}"
echo "  Total time: ${DURATION}s"
echo "  Average response time: ${AVG_TIME}ms"
echo "  Requests per second: ${RPS}"

# Final summary
echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}Test Summary${NC}"
echo -e "${BLUE}========================================${NC}"

FINAL_STATS=$(curl -s "${BASE_URL}/stats")
echo "$FINAL_STATS" | python3 -m json.tool

CACHE_ENABLED=$(echo "$FINAL_STATS" | python3 -c "import sys, json; print(json.load(sys.stdin)['cache_enabled'])")
HIT_RATE=$(echo "$FINAL_STATS" | python3 -c "import sys, json; print(json.load(sys.stdin).get('cache_hit_rate', 0) * 100)")

echo ""
if [ "$CACHE_ENABLED" == "True" ] || [ "$CACHE_ENABLED" == "true" ]; then
    echo -e "${GREEN}✓ Cache is ENABLED${NC}"
    echo -e "${GREEN}✓ Cache hit rate: ${HIT_RATE}%${NC}"

    if (( $(awk "BEGIN {print ($HIT_RATE > 50)}") )); then
        echo -e "${GREEN}✓ Good cache hit rate!${NC}"
    else
        echo -e "${YELLOW}⚠ Low cache hit rate. Consider testing with more repeated IPs.${NC}"
    fi
else
    echo -e "${RED}✗ Cache is DISABLED${NC}"
fi

echo -e "\n${BLUE}Test completed!${NC}"
