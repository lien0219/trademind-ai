#!/usr/bin/env bash

# Load dotenv assignments only when the process environment does not already
# define the key. Values are not evaluated or echoed.
load_env_defaults() {
  local env_file="$1"
  local line key value first last
  local -A file_values=()

  [[ -f "$env_file" ]] || { echo "environment file not found: $env_file" >&2; return 1; }

  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%$'\r'}"
    [[ "$line" =~ ^[[:space:]]*$ || "$line" =~ ^[[:space:]]*# ]] && continue
    [[ "$line" =~ ^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*=(.*)$ ]] || continue

    key="${BASH_REMATCH[1]}"
    value="${BASH_REMATCH[2]}"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    if (( ${#value} >= 2 )); then
      first="${value:0:1}"
      last="${value: -1}"
      if [[ ( "$first" == '"' && "$last" == '"' ) || ( "$first" == "'" && "$last" == "'" ) ]]; then
        value="${value:1:${#value}-2}"
      fi
    fi

    file_values["$key"]="$value"
  done < "$env_file"

  # Apply the last file assignment, as Compose and Node preflight do.
  for key in "${!file_values[@]}"; do
    [[ -v $key ]] && continue
    printf -v "$key" '%s' "${file_values[$key]}"
    export "$key"
  done
}
