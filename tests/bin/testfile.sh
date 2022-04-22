#!/bin/sh

if [ -f "$1" ]; then
    echo "file exists"
    exit 0
fi
echo "file not found"
exit 1