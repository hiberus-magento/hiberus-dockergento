#!/usr/bin/env bash

echo $(jq -r '.["'$1'"]' < "$DATA_DIR/equivalent_versions.json")