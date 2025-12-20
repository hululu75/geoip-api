# Cache Testing Guide

This directory contains two test scripts for validating the GeoIP API cache functionality.

## Important Note on Performance

**Cache performance varies significantly based on your environment:**

- ✅ **Cache helps when:** High traffic (>1000 req/s), repeated IPs, remote/slow storage
- ❌ **Cache may hurt when:** Low traffic (<100 req/s), unique IPs, local SSD with OS caching

**Your test results may show cache is slower!** This is normal for:
- Local testing (database already in OS memory cache)
- Low traffic scenarios
- Fast SSD storage

Always test with your actual production workload and environment.

## Test Scripts

### 1. `test_cache.sh` - Complete Performance Test

**Purpose:** Automatically starts/stops the application and compares performance with cache enabled vs disabled.

**Usage:**
```bash
./test_cache.sh
```

**Environment Variables:**
```bash
PORT=12345                                            # API port (default: 12345)
GEOIP_DB_PATH=/root/docker/geoip/GeoLite2-Country.mmdb  # Database path
CACHE_MAX_SIZE=1000                                   # Cache size (default: 1000)

# Example:
PORT=8080 CACHE_MAX_SIZE=5000 ./test_cache.sh
```

**What it does:**
1. Starts app with cache **ENABLED**
2. Runs performance test (100 iterations × 10 IPs = 1000 requests)
3. Stops app
4. Starts app with cache **DISABLED**
5. Runs same performance test
6. Compares results and shows speedup

**Sample Output:**
```
========================================
Performance Comparison
========================================
Cache ON:
  Requests/sec: 1542.38
  Avg response: 0.6483ms

Cache OFF:
  Requests/sec: 456.12
  Avg response: 2.1926ms

Improvement:
  Speedup: 3.38x faster
  Response time improvement: 70.43%

✓ Cache provides significant performance improvement!
```

---

### 2. `test_cache_quick.sh` - Quick Test for Running App

**Purpose:** Test cache functionality on an already running application.

**Usage:**
```bash
# Test app running on port 12345 (default)
./test_cache_quick.sh

# Test app on different port
./test_cache_quick.sh 8080
```

**What it does:**
1. Verifies app is running
2. Tests cache hits with repeated requests
3. Tests cache misses with new IPs
4. Simulates realistic mixed usage pattern
5. Measures performance
6. Shows final cache statistics

**Sample Output:**
```
=== Test 1: Repeated Requests (Cache Hit Test) ===
Sending 10 requests to 8.8.8.8...
Expected: 9 cache hits (first miss, then 9 hits)
Actual cache hits: 9

=== Test 4: Performance Test ===
Performance Results:
  Total time: 0.0648s
  Average response time: 0.6480ms
  Requests per second: 1543.21

✓ Cache is ENABLED
✓ Cache hit rate: 72.5%
✓ Good cache hit rate!
```

---

## Quick Start

### Option 1: Quick Test (App Already Running)

If you have the app running:
```bash
# Start the app first
export GEOIP_DB_PATH=/root/docker/geoip/GeoLite2-Country.mmdb
export CACHE_ENABLED=true
export CACHE_MAX_SIZE=100
export PORT=12345
go run main.go &

# Run quick test
./test_cache_quick.sh 12345
```

### Option 2: Full Performance Comparison

Automatically tests both cache enabled and disabled:
```bash
./test_cache.sh
```

---

## Manual Testing

### Test Cache Enabled
```bash
# Start with cache
export CACHE_ENABLED=true
export CACHE_MAX_SIZE=100
export PORT=12345
go run main.go &

# Test repeated requests (should hit cache)
curl http://localhost:12345/country/8.8.8.8  # First: cache miss
curl http://localhost:12345/country/8.8.8.8  # Second: cache hit ✓
curl http://localhost:12345/country/8.8.8.8  # Third: cache hit ✓

# Check statistics
curl http://localhost:12345/stats | python3 -m json.tool
```

Expected stats:
```json
{
    "cache_enabled": true,
    "cache_hits": 2,
    "cache_misses": 1,
    "cache_size": 1,
    "cache_hit_rate": 0.6666
}
```

### Test Cache Disabled
```bash
# Start without cache
export CACHE_ENABLED=false
export PORT=12345
go run main.go &

# Test repeated requests (no cache benefit)
curl http://localhost:12345/country/8.8.8.8  # Query database
curl http://localhost:12345/country/8.8.8.8  # Query database again
curl http://localhost:12345/country/8.8.8.8  # Query database again

# Check statistics
curl http://localhost:12345/stats | python3 -m json.tool
```

Expected stats:
```json
{
    "cache_enabled": false,
    "cache_hits": 0,
    "cache_misses": 3
}
```

---

## Understanding Cache Behavior

### LRU (Least Recently Used) Algorithm

When cache is full (e.g., 100 items), adding a new item automatically removes the **least recently used** item.

**Example with CACHE_MAX_SIZE=3:**
```
Cache: []
Request 8.8.8.8  → Cache: [8.8.8.8]
Request 1.1.1.1  → Cache: [8.8.8.8, 1.1.1.1]
Request 9.9.9.9  → Cache: [8.8.8.8, 1.1.1.1, 9.9.9.9]  (full)

Request 8.8.8.8  → Cache: [1.1.1.1, 9.9.9.9, 8.8.8.8]  (moved to end)
Request 2.2.2.2  → Cache: [9.9.9.9, 8.8.8.8, 2.2.2.2]  (1.1.1.1 evicted)
                                     ^^^
                                     LRU
```

### Expected Performance

**Cache Hit (from memory):**
- Response time: ~0.5-1ms
- No database query

**Cache Miss (database query):**
- Response time: ~2-5ms
- Queries GeoIP database

**Typical speedup:** 2-5x faster with cache (depends on cache hit rate)

---

## Troubleshooting

### Port Already in Use
```bash
# Find process using port
ss -tlnp | grep :12345

# Kill process
kill <PID>

# Or use different port
PORT=9999 ./test_cache.sh
```

### Database Not Found
```bash
# Check database exists
ls -lh /root/docker/geoip/GeoLite2-Country.mmdb

# Set correct path
export GEOIP_DB_PATH=/path/to/your/GeoLite2-Country.mmdb
```

### Tests Fail
```bash
# Check if app is running
curl http://localhost:12345/health

# View app logs
tail -f /tmp/geoip-test.log

# Check cache status
curl http://localhost:12345/stats | python3 -m json.tool
```

---

## CI/CD Integration

Add to your CI pipeline:
```yaml
# .github/workflows/test.yml
- name: Run cache performance test
  run: |
    chmod +x test_cache.sh
    ./test_cache.sh
```

---

## Tips

1. **Default is disabled:** Cache is disabled by default for optimal performance in most scenarios

2. **Benchmark first:** Always run `./test_cache.sh` before enabling cache in production
   - If speedup < 1.5x, keep cache disabled
   - If speedup > 2x, enable cache

3. **Understand the results:**
   - **Cache slower?** Normal for local SSD + low traffic
   - **Cache faster?** You have high traffic or repeated IPs

4. **For production:** Set `CACHE_MAX_SIZE` based on your traffic
   - 1000 items ≈ 100KB memory
   - 10000 items ≈ 1MB memory
   - 100000 items ≈ 10MB memory

5. **Monitor cache hit rate:** Enable cache only if hit rate >70%

6. **When to enable cache:**
   - High traffic websites (>1000 req/s)
   - CDN or proxy scenarios with repeated IPs
   - Remote storage or network-attached database

7. **When to keep cache disabled:**
   - Personal projects or development
   - Low traffic (<100 req/s)
   - Unique IPs (log processing, batch analytics)
   - Local SSD with fast mmdb access
