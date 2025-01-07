#!/bin/bash
# Script to reset the data/schemas directory by cleaning its contents

# Check if the directory exists
if [ -d "src/data/schemas" ]; then
  echo "Cleaning the data/schemas directory..."
  rm -rf src/data/schemas/*
  echo "Reset complete. The directory is now clean."
else
  echo "Directory 'data/schemas' does not exist."
fi
