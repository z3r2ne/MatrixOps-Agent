#!/bin/bash
# Generate macOS .icns file from a 1024x1024 PNG

ICON_FILE="frontend/build/icon.png"
ICON_SET="frontend/build/icon.iconset"

mkdir -p $ICON_SET

# Create all necessary sizes
sips -z 16 16     $ICON_FILE --out $ICON_SET/icon_16x16.png
sips -z 32 32     $ICON_FILE --out $ICON_SET/icon_16x16@2x.png
sips -z 32 32     $ICON_FILE --out $ICON_SET/icon_32x32.png
sips -z 64 64     $ICON_FILE --out $ICON_SET/icon_32x32@2x.png
sips -z 128 128   $ICON_FILE --out $ICON_SET/icon_128x128.png
sips -z 256 256   $ICON_FILE --out $ICON_SET/icon_128x128@2x.png
sips -z 256 256   $ICON_FILE --out $ICON_SET/icon_256x256.png
sips -z 512 512   $ICON_FILE --out $ICON_SET/icon_256x256@2x.png
sips -z 512 512   $ICON_FILE --out $ICON_SET/icon_512x512.png
sips -z 1024 1024 $ICON_FILE --out $ICON_SET/icon_512x512@2x.png

# Convert iconset to icns
iconutil -c icns $ICON_SET

# Clean up
rm -rf $ICON_SET

echo "Generated frontend/build/icon.icns"
