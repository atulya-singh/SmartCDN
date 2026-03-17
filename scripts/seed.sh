#!/usr/bin/env bash
set -euo pipefail

# Upload sample images to the running SmartCDN server for testing.
# Downloads 5 images of varying sizes from picsum.photos and uploads
# each via POST /upload.

BASE_URL="${SMARTCDN_URL:-http://localhost:8080}"

SIZES=("800/600" "1920/1080" "640/480" "1200/900" "400/300")
LABELS=("small-landscape" "full-hd" "vga" "medium" "thumbnail")

echo "Seeding SmartCDN at ${BASE_URL} with sample images..."
echo ""

IMAGE_IDS=()

for i in "${!SIZES[@]}"; do
    size="${SIZES[$i]}"
    label="${LABELS[$i]}"
    tmpfile=$(mktemp /tmp/seed_XXXXXX.jpg)

    echo "Downloading ${label} (${size})..."
    curl -sL "https://picsum.photos/${size}" -o "${tmpfile}"

    filesize=$(wc -c < "${tmpfile}" | tr -d ' ')
    echo "  Downloaded ${filesize} bytes"

    echo "  Uploading to ${BASE_URL}/upload..."
    response=$(curl -s -X POST "${BASE_URL}/upload" \
        -F "image=@${tmpfile};type=image/jpeg")

    id=$(echo "${response}" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "${id}" ]; then
        echo "  ERROR: Upload failed. Response: ${response}"
        rm -f "${tmpfile}"
        continue
    fi

    IMAGE_IDS+=("${id}")
    echo "  Image ID: ${id}"
    echo ""

    rm -f "${tmpfile}"
done

echo "=== Seeding Complete ==="
echo "Uploaded ${#IMAGE_IDS[@]} images."
echo ""
echo "Image IDs:"
for id in "${IMAGE_IDS[@]}"; do
    echo "  ${id}"
done

echo ""
echo "Test with:"
for id in "${IMAGE_IDS[@]}"; do
    echo "  curl -s -o /dev/null -w '%{http_code}' ${BASE_URL}/img/${id}"
    break
done
