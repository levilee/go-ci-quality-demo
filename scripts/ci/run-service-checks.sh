#!/usr/bin/env bash
set -euo pipefail

service_path="${1:?service path is required}"
language="${2:?language is required}"
coverage_threshold="${SERVICE_COVERAGE_THRESHOLD:-70}"

cd "$service_path"

require_package_script() {
  local script_name="$1"
  node -e 'const p = require("./package.json"); process.exit(p.scripts && p.scripts[process.argv[1]] ? 0 : 1)' "$script_name" \
    || { echo "package.json must define a ${script_name} script" >&2; exit 1; }
}

case "$language" in
  go)
    mapfile -t go_files < <(git ls-files -- '*.go')
    if [[ ${#go_files[@]} -eq 0 ]]; then
      echo 'No tracked Go source files found for the declared Go service.' >&2
      exit 1
    fi
    files="$(gofmt -l "${go_files[@]}")"
    if [[ -n "$files" ]]; then
      echo 'The following files need gofmt:' >&2
      echo "$files" >&2
      exit 1
    fi
    go vet ./...
    go test -race -covermode=atomic -coverprofile=coverage.out ./...
    coverage="$(go tool cover -func=coverage.out | awk '/^total:/ {gsub("%", "", $3); print $3}')"
    echo "Total coverage: ${coverage}% (required: ${coverage_threshold}%)"
    awk -v coverage="$coverage" -v threshold="$coverage_threshold" 'BEGIN { exit !(coverage >= threshold) }'
    go build -buildvcs=false ./...
    ;;
  node)
    corepack enable
    if [[ -f pnpm-lock.yaml ]]; then
      pnpm install --frozen-lockfile
      require_package_script lint
      require_package_script test
      require_package_script build
      pnpm lint
      pnpm test -- --coverage
      pnpm build
    elif [[ -f package-lock.json ]]; then
      npm ci
      require_package_script lint
      require_package_script test
      require_package_script build
      npm run lint
      npm test -- --coverage
      npm run build
    elif [[ -f yarn.lock ]]; then
      yarn install --immutable
      require_package_script lint
      require_package_script test
      require_package_script build
      yarn lint
      yarn test --coverage
      yarn build
    else
      echo 'Node.js services require pnpm-lock.yaml, package-lock.json, or yarn.lock.' >&2
      exit 1
    fi
    ;;
  java)
    if [[ -x ./mvnw || -f ./mvnw ]]; then
      chmod +x ./mvnw
      ./mvnw -B -ntp verify
    elif [[ -x ./gradlew || -f ./gradlew ]]; then
      chmod +x ./gradlew
      ./gradlew --no-daemon check build
    elif [[ -f pom.xml ]]; then
      mvn -B -ntp verify
    elif [[ -f build.gradle || -f build.gradle.kts ]]; then
      gradle --no-daemon check build
    else
      echo 'Java services require a Maven or Gradle build descriptor.' >&2
      exit 1
    fi
    ;;
  *)
    echo "Unsupported language: $language" >&2
    exit 1
    ;;
esac
