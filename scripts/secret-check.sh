#!/usr/bin/env bash

if [[ "${BASH_SOURCE[0]}" != "${0}" ]]; then
  echo "scripts/secret-check.sh must be executed, not sourced" >&2
  return 1 2>/dev/null || exit 1
fi

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

VALUE_CHARS='[A-Za-z0-9%._:+/=-]'

COOKIE_ASSIGN_PATTERN="(^|[^[:alnum:]_])(GAT|csrfToken|_sid|iget)[[:space:]]*(:=|=|:)[[:space:]]*[\"']${VALUE_CHARS}{16,}[\"']"
COOKIE_SERIALIZED_PATTERN="(^|[;,{[:space:]\"'])(GAT|csrfToken|_sid|iget)=${VALUE_CHARS}{16,}([;,\"']|[[:space:]]|$)"
JWT_PATTERN='(^|[^[:alnum:]_-])eyJ[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}($|[^[:alnum:]_-])'
GO_ASSIGN_PATTERN='(^|[^[:alnum:]_])(var[[:space:]]+)?[[:alpha:]_][[:alnum:]_]*(Cookie|Token|Sign|cookie|token|sign)[[:alnum:]_]*[[:space:]]*(:=|=)[[:space:]]*"[^"]{16,}"'
PHONE_LITERAL_PATTERN='["'\'']1[3-9][0-9]{9}["'\'']'

PATHS=(
  backend
  frontend
  scripts
  .github
  ':(exclude)scripts/secret-check.sh'
  ':(exclude)scripts/secret-check_test.sh'
  ':(exclude,glob)frontend/src/assets/**'
)

GREP_PATTERNS=(
  -e "${COOKIE_ASSIGN_PATTERN}"
  -e "${COOKIE_SERIALIZED_PATTERN}"
  -e "${JWT_PATTERN}"
  -e "${GO_ASSIGN_PATTERN}"
  -e "${PHONE_LITERAL_PATTERN}"
)

usage() {
  cat <<'EOF'
Usage: scripts/secret-check.sh [--history]
EOF
}

normalize_hits() {
  printf '%s\n' "${1:-}" | sed '/^$/d'
}

capture_grep() {
  local __var_name="$1"
  shift
  local output
  local status

  set +e
  output="$("$@")"
  status=$?
  set -e

  case "${status}" in
    0|1)
      printf -v "${__var_name}" '%s' "${output}"
      return "${status}"
      ;;
    *)
      return "${status}"
      ;;
  esac
}

scan_worktree() {
  local hits
  local status

  if capture_grep hits git grep -IlE --untracked --exclude-standard "${GREP_PATTERNS[@]}" -- "${PATHS[@]}"; then
    status=0
  else
    status=$?
  fi

  case "${status}" in
    1)
      return 0
      ;;
    0)
      hits="$(normalize_hits "${hits}")"
      if [[ -z "${hits}" ]]; then
        return 0
      fi
      printf 'working-tree\n%s\n' "${hits}"
      return 1
      ;;
    *)
      return "${status}"
      ;;
  esac
}

scan_history() {
  local commit
  local hits
  local status
  local found=0

  while IFS= read -r commit; do
    if capture_grep hits git grep -IlE "${GREP_PATTERNS[@]}" "${commit}" -- "${PATHS[@]}"; then
      status=0
    else
      status=$?
    fi

    case "${status}" in
      1)
        continue
        ;;
      0)
        hits="$(normalize_hits "${hits}")"
        if [[ -z "${hits}" ]]; then
          continue
        fi
        hits="$(printf '%s\n' "${hits}" | sed "s#^${commit}:##")"
        found=1
        printf '%s\n%s\n' "${commit}" "${hits}"
        ;;
      *)
        return "${status}"
        ;;
    esac
  done < <(git rev-list --all)

  if [[ "${found}" -eq 0 ]]; then
    return 0
  fi

  return 1
}

main() {
  cd "${ROOT_DIR}"

  case "${1:-}" in
    "")
      scan_worktree
      ;;
    --history)
      scan_history
      ;;
    -h|--help)
      usage
      ;;
    *)
      usage >&2
      return 1
      ;;
  esac
}

main "$@"
