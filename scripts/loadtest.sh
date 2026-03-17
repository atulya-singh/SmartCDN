#!/usr/bin/env bash
set -euo pipefail

# Load test SmartCDN with different device User-Agents.
# Requires: hey (go install github.com/rakyll/hey@latest)
#
# Usage: ./scripts/loadtest.sh <image_id>
#   Or set IMAGE_ID env var. If neither provided, script uploads a test image first.

BASE_URL="${SMARTCDN_URL:-http://localhost:8080}"
REQUESTS="${REQUESTS:-1000}"
CONCURRENCY="${CONCURRENCY:-50}"

# User-Agent strings for each device type
UA_IPHONE="Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
UA_OLD_ANDROID="Mozilla/5.0 (Linux; Android 4.4.2; SM-G900F Build/KOT49H) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/45.0.2454.94 Mobile Safari/537.36"
UA_IPAD="Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
UA_DESKTOP="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

# Resolve image ID
IMAGE_ID="${1:-${IMAGE_ID:-}}"
if [ -z "${IMAGE_ID}" ]; then
    echo "No image ID provided. Uploading a test image..."
    # Generate a simple test JPEG with ImageMagick if available, otherwise download one
    tmpfile=$(mktemp /tmp/loadtest_XXXXXX.jpg)
    if command -v convert &> /dev/null; then
        convert -size 1920x1080 xc:blue -quality 90 "${tmpfile}"
    else
        curl -sL "https://picsum.photos/1920/1080" -o "${tmpfile}"
    fi
    response=$(curl -s -X POST "${BASE_URL}/upload" -F "image=@${tmpfile};type=image/jpeg")
    IMAGE_ID=$(echo "${response}" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    rm -f "${tmpfile}"
    if [ -z "${IMAGE_ID}" ]; then
        echo "ERROR: Failed to upload test image. Response: ${response}"
        exit 1
    fi
    echo "Uploaded test image: ${IMAGE_ID}"
fi

URL="${BASE_URL}/img/${IMAGE_ID}"

echo "=== SmartCDN Load Test ==="
echo "URL:         ${URL}"
echo "Requests:    ${REQUESTS} per device type"
echo "Concurrency: ${CONCURRENCY}"
echo ""

# Check if hey is installed
if ! command -v hey &> /dev/null; then
    echo "'hey' not found. Install with: go install github.com/rakyll/hey@latest"
    echo "Falling back to curl-based load test..."
    echo ""

    run_curl_test() {
        local label="$1"
        local ua="$2"
        local hits=0
        local misses=0
        local total=0
        local start_time end_time

        echo "Testing: ${label}..."
        start_time=$(date +%s%N)

        for i in $(seq 1 "${REQUESTS}"); do
            cache_header=$(curl -s -o /dev/null -w '%{http_code}' \
                -H "User-Agent: ${ua}" \
                -H "Accept: image/webp,image/jpeg,*/*" \
                -D - "${URL}" 2>/dev/null | grep -i "X-SmartCDN-Cache" | tr -d '\r' | awk '{print $2}')
            if [ "${cache_header}" = "HIT" ]; then
                hits=$((hits + 1))
            else
                misses=$((misses + 1))
            fi
            total=$((total + 1))
        done

        end_time=$(date +%s%N)
        duration=$(( (end_time - start_time) / 1000000 ))
        avg_latency=$((duration / total))
        hit_rate=0
        if [ "${total}" -gt 0 ]; then
            hit_rate=$(( (hits * 100) / total ))
        fi

        echo "  Total: ${total} | Avg latency: ${avg_latency}ms | Cache HIT: ${hits} (${hit_rate}%) | MISS: ${misses}"
    }

    run_curl_test "iPhone Safari"   "${UA_IPHONE}"
    run_curl_test "Old Android"     "${UA_OLD_ANDROID}"
    run_curl_test "iPad"            "${UA_IPAD}"
    run_curl_test "Chrome Desktop"  "${UA_DESKTOP}"
    exit 0
fi

# Use hey for proper load testing
RESULTS_DIR=$(mktemp -d /tmp/loadtest_results_XXXXXX)

echo "Running load tests with hey..."
echo ""

run_hey_test() {
    local label="$1"
    local ua="$2"
    local outfile="${RESULTS_DIR}/${label// /_}.txt"

    echo "Testing: ${label}..."
    hey -n "${REQUESTS}" -c "${CONCURRENCY}" \
        -H "User-Agent: ${ua}" \
        -H "Accept: image/webp,image/jpeg,*/*" \
        "${URL}" > "${outfile}" 2>&1

    echo "  Done."
}

# Run all 4 device types in parallel
run_hey_test "iPhone Safari"   "${UA_IPHONE}" &
run_hey_test "Old Android"     "${UA_OLD_ANDROID}" &
run_hey_test "iPad"            "${UA_IPAD}" &
run_hey_test "Chrome Desktop"  "${UA_DESKTOP}" &
wait

echo ""
echo "=== Results ==="
echo ""

total_requests=0

for result_file in "${RESULTS_DIR}"/*.txt; do
    label=$(basename "${result_file}" .txt | tr '_' ' ')
    echo "--- ${label} ---"

    # Extract summary stats from hey output
    rps=$(grep "Requests/sec:" "${result_file}" | awk '{print $2}')
    avg=$(grep "Average:" "${result_file}" | head -1 | awk '{print $2}')
    p99=$(grep "99%" "${result_file}" | head -1 | awk '{print $2}' || echo "N/A")
    total=$(grep "^\s*Total:" "${result_file}" | head -1 | awk '{print $2}' || echo "N/A")
    success=$(grep "\[200\]" "${result_file}" | awk '{print $2}' || echo "0")

    echo "  Requests/sec: ${rps:-N/A}"
    echo "  Avg latency:  ${avg:-N/A}"
    echo "  P99 latency:  ${p99:-N/A}"
    echo "  Total time:   ${total:-N/A}"
    echo "  Success:      ${success:-N/A}"
    echo ""

    total_requests=$((total_requests + REQUESTS))
done

echo "=== Summary ==="
echo "Total requests across all device types: ${total_requests}"
echo ""

# Cache hit rate check via curl sampling
echo "Cache hit rate sampling (10 requests per device)..."
total_hits=0
total_samples=0

for ua in "${UA_IPHONE}" "${UA_OLD_ANDROID}" "${UA_IPAD}" "${UA_DESKTOP}"; do
    for i in $(seq 1 10); do
        cache_header=$(curl -s -D - -o /dev/null \
            -H "User-Agent: ${ua}" \
            -H "Accept: image/webp,image/jpeg,*/*" \
            "${URL}" 2>/dev/null | grep -i "X-SmartCDN-Cache" | tr -d '\r' | awk '{print $2}')
        if [ "${cache_header}" = "HIT" ]; then
            total_hits=$((total_hits + 1))
        fi
        total_samples=$((total_samples + 1))
    done
done

hit_rate=$(( (total_hits * 100) / total_samples ))
echo "  Sampled ${total_samples} requests: ${total_hits} HITs (${hit_rate}%)"

# Cleanup
rm -rf "${RESULTS_DIR}"
