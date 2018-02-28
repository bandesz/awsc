#!/bin/sh

set -e

for var in "$@"; do
  printf "${var},"
done

echo "${ENV_VAR_1},${ENV_VAR_2}"
