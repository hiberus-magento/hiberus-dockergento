#!/usr/bin/env bash

echo $(cat "${DATA_DIR}/equivalent_versions.json" | jq -r '.['\"$1\"']')