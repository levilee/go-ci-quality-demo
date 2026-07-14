#!/usr/bin/env bash
set -euo pipefail

# Service boundary convention:
# - A service owns one of go.mod, package.json, pom.xml, build.gradle, or build.gradle.kts.
# - New microservices belong in services/<service-name>/.
# - A root-level manifest represents the root application only; changes below services/
#   do not automatically retest it.
# - Changes to shared directories or CI definitions test every discovered service.

base_sha="${1:-}"
head_sha="${2:-HEAD}"
repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

declare -a service_keys=()
declare -A service_seen=()
declare -A service_path=()
declare -A service_language=()
declare -A selected=()
declare -a service_entries=()
declare -a go_service_entries=()

json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '%s' "$value"
}

json_array() {
  local IFS=,
  printf '%s' "$*"
}

add_service() {
  local path="$1"
  local language="$2"
  local key="${path}|${language}"

  if [[ -n "${service_seen[$key]:-}" ]]; then
    return
  fi

  service_seen["$key"]=1
  service_path["$key"]="$path"
  service_language["$key"]="$language"
  service_keys+=("$key")
}

while IFS= read -r manifest; do
  manifest="${manifest#./}"
  directory="$(dirname "$manifest")"
  case "$(basename "$manifest")" in
    go.mod)
      add_service "$directory" go
      ;;
    package.json)
      add_service "$directory" node
      ;;
    pom.xml|build.gradle|build.gradle.kts)
      add_service "$directory" java
      ;;
  esac
done < <(find . -type f \( -name go.mod -o -name package.json -o -name pom.xml -o -name build.gradle -o -name build.gradle.kts \) \
  -not -path './.git/*' -not -path './.gocache/*' -not -path './.gomodcache/*' -not -path './.tools/*' \
  -not -path '*/node_modules/*' -not -path '*/vendor/*' | sort)

if [[ ${#service_keys[@]} -eq 0 ]]; then
  echo 'No service manifest found. A service must contain go.mod, package.json, pom.xml, or build.gradle.' >&2
  exit 1
fi

run_all=false
changed_files=()
if [[ -z "$base_sha" || "$base_sha" =~ ^0+$ ]] || ! git cat-file -e "${base_sha}^{commit}" 2>/dev/null; then
  run_all=true
else
  mapfile -t changed_files < <(git diff --name-only "$base_sha" "$head_sha")
  if [[ ${#changed_files[@]} -eq 0 ]]; then
    run_all=true
  fi
fi

if [[ "$run_all" != true ]]; then
  for changed in "${changed_files[@]}"; do
    matched=false

    case "$changed" in
      .github/workflows/*|scripts/ci/*|ci/*|libs/*|shared/*|go.work|go.work.sum)
        run_all=true
        break
        ;;
    esac

    for key in "${service_keys[@]}"; do
      path="${service_path[$key]}"
      if [[ "$path" == '.' ]]; then
        if [[ "$changed" != services/* ]]; then
          selected["$key"]=1
          matched=true
        fi
      elif [[ "$changed" == "$path" || "$changed" == "$path"/* ]]; then
        selected["$key"]=1
        matched=true
      fi
    done

    # A change outside every declared service has unknown consumers, so safely fan out.
    if [[ "$matched" != true ]]; then
      run_all=true
      break
    fi
  done
fi

service_count=0
go_service_count=0

for key in "${service_keys[@]}"; do
  if [[ "$run_all" != true && -z "${selected[$key]:-}" ]]; then
    continue
  fi

  path="${service_path[$key]}"
  language="${service_language[$key]}"
  if [[ "$path" == '.' ]]; then
    base_name="$(basename "$repo_root")"
  else
    base_name="$(basename "$path")"
  fi
  name="$(printf '%s-%s' "$base_name" "$language" | tr -cs '[:alnum:]_.-' '-')"

  entry="$(printf '{\"service\":{\"name\":\"%s\",\"path\":\"%s\",\"language\":\"%s\"}}' \
    "$(json_escape "$name")" "$(json_escape "$path")" "$(json_escape "$language")")"
  service_entries+=("$entry")
  ((service_count += 1))

  if [[ "$language" == go ]]; then
    go_service_entries+=("$entry")
    ((go_service_count += 1))
  fi
done

services="{\"include\":[$(json_array "${service_entries[@]}")]}"
go_services="{\"include\":[$(json_array "${go_service_entries[@]}")]}"

output_file="${GITHUB_OUTPUT:-/dev/stdout}"
{
  echo "services=$services"
  echo "go-services=$go_services"
  if [[ "$service_count" -gt 0 ]]; then
    echo 'has-services=true'
  else
    echo 'has-services=false'
  fi
  if [[ "$go_service_count" -gt 0 ]]; then
    echo 'has-go-services=true'
  else
    echo 'has-go-services=false'
  fi
} >> "$output_file"
