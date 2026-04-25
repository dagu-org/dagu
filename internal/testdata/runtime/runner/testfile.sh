#!/bin/sh

test -f "$1" || {
    echo "file not found"
    exit 1
}

echo "file exists"
