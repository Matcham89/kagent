#!/bin/bash

# Base directory for all agents
AGENTS_DIR="../agents"

# Find all Chart-template.yaml files and rename them to Chart.yaml
find "$AGENTS_DIR" -type f -name "Chart-template.yaml" | while read -r file; do
  chart_dir=$(dirname "$file")
  echo "Renaming $file to $chart_dir/Chart.yaml"
  mv "$file" "$chart_dir/Chart.yaml"
done

VERSION="0.3.6"
find ../agents -name 'Chart.yaml' | while read -r file; do
  echo "Setting version in $file"
  yq e ".version = \"$VERSION\"" -i "$file"
done
