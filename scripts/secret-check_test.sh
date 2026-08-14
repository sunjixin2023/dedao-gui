#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
CHECKER_SOURCE="${SCRIPT_DIR}/secret-check.sh"
TEMP_PARENT="${TMPDIR:-/tmp}"
TEMP_PARENT="${TEMP_PARENT%/}"

TEMP_ROOT=""
TEST_REPO=""
COPIED_CHECKER=""

cleanup() {
  if [[ -n "${TEMP_ROOT}" ]]; then
    case "${TEMP_ROOT}" in
      "${TEMP_PARENT}"/secret-check-test.*|/tmp/secret-check-test.*)
        rm -rf "${TEMP_ROOT}"
        ;;
      *)
        echo "refusing to remove unexpected temp path: ${TEMP_ROOT}" >&2
        exit 1
        ;;
    esac
  fi
}

make_temp_repo() {
  TEMP_ROOT="$(mktemp -d "${TEMP_PARENT}/secret-check-test.XXXXXX")"
  case "${TEMP_ROOT}" in
    "${TEMP_PARENT}"/secret-check-test.*|/tmp/secret-check-test.*)
      ;;
    *)
      echo "unexpected temp path: ${TEMP_ROOT}" >&2
      exit 1
      ;;
  esac

  TEST_REPO="${TEMP_ROOT}/repo"
  COPIED_CHECKER="${TEST_REPO}/scripts/secret-check.sh"

  mkdir -p \
    "${TEST_REPO}/scripts" \
    "${TEST_REPO}/backend" \
    "${TEST_REPO}/frontend" \
    "${TEST_REPO}/frontend/src/assets" \
    "${TEST_REPO}/.github"

  git -C "${TEST_REPO}" init -q
  cp "${CHECKER_SOURCE}" "${COPIED_CHECKER}"
  chmod +x "${COPIED_CHECKER}"
}

reset_repo_tree() {
  rm -rf \
    "${TEST_REPO}/backend" \
    "${TEST_REPO}/frontend" \
    "${TEST_REPO}/.github"

  mkdir -p \
    "${TEST_REPO}/backend" \
    "${TEST_REPO}/frontend" \
    "${TEST_REPO}/frontend/src/assets" \
    "${TEST_REPO}/.github"
}

write_repo_file() {
  local rel_path="$1"
  mkdir -p "$(dirname "${TEST_REPO}/${rel_path}")"
  cat > "${TEST_REPO}/${rel_path}"
}

run_checker() {
  (
    cd "${TEST_REPO}"
    bash "${COPIED_CHECKER}"
  )
}

assert_clean_repo() {
  local label="$1"
  local output

  if ! output="$(run_checker 2>&1)"; then
    echo "expected clean repo for ${label}"
    exit 1
  fi

  if [[ -n "${output}" ]]; then
    echo "expected no output for clean repo ${label}"
    exit 1
  fi
}

assert_rejected_repo() {
  local label="$1"
  local expected_path="$2"
  local output

  if output="$(run_checker 2>&1)"; then
    echo "expected checker to reject ${label}"
    exit 1
  fi

  if [[ "${output}" != $'working-tree\n'"${expected_path}" ]]; then
    echo "unexpected checker output for ${label}"
    exit 1
  fi
}

assert_source_rejected_without_mutation() {
  local output

  output="$(
    CHECKER_PATH="${COPIED_CHECKER}" bash <<'EOF'
set +e
set +u
set +o pipefail

before_opts="$(set -o | awk '/errexit|nounset|pipefail/ {print $1 "=" $2}')"
before_functions="$(declare -F capture_grep main normalize_hits scan_history scan_worktree usage 2>/dev/null || true)"
before_globals="$(compgen -A variable | LC_ALL=C grep -E '^(COOKIE_ASSIGN_PATTERN|COOKIE_SERIALIZED_PATTERN|GO_ASSIGN_PATTERN|GREP_PATTERNS|JWT_PATTERN|PATHS|PHONE_LITERAL_PATTERN|ROOT_DIR|SCRIPT_DIR|VALUE_CHARS)$' || true)"

. "${CHECKER_PATH}" >/dev/null 2>&1
status=$?

after_opts="$(set -o | awk '/errexit|nounset|pipefail/ {print $1 "=" $2}')"
after_functions="$(declare -F capture_grep main normalize_hits scan_history scan_worktree usage 2>/dev/null || true)"
after_globals="$(compgen -A variable | LC_ALL=C grep -E '^(COOKIE_ASSIGN_PATTERN|COOKIE_SERIALIZED_PATTERN|GO_ASSIGN_PATTERN|GREP_PATTERNS|JWT_PATTERN|PATHS|PHONE_LITERAL_PATTERN|ROOT_DIR|SCRIPT_DIR|VALUE_CHARS)$' || true)"

printf 'status=%s\n' "${status}"
printf 'before_opts=%s\n' "${before_opts}"
printf 'after_opts=%s\n' "${after_opts}"
printf 'before_functions=%s\n' "${before_functions}"
printf 'after_functions=%s\n' "${after_functions}"
printf 'before_globals=%s\n' "${before_globals}"
printf 'after_globals=%s\n' "${after_globals}"
EOF
  )"

  case "${output}" in
    *$'status=1'*)
      ;;
    *)
      echo "expected sourcing to return nonzero"
      exit 1
      ;;
  esac

  case "${output}" in
    *$'before_opts=errexit=off\nnounset=off\npipefail=off\nafter_opts=errexit=off\nnounset=off\npipefail=off'*)
      ;;
    *)
      echo "sourcing checker changed shell options"
      exit 1
      ;;
  esac

  case "${output}" in
    *$'before_functions=\nafter_functions='*)
      ;;
    *)
      echo "sourcing checker defined functions"
      exit 1
      ;;
  esac

  case "${output}" in
    *$'before_globals=\nafter_globals='*)
      ;;
    *)
      echo "sourcing checker defined globals"
      exit 1
      ;;
  esac
}

trap cleanup EXIT

if [[ ! -x "${CHECKER_SOURCE}" ]]; then
  echo "missing executable checker: ${CHECKER_SOURCE}"
  exit 1
fi

make_temp_repo
assert_source_rejected_without_mutation
assert_clean_repo baseline

cookie_left='alpha012345'
cookie_right='6789+/='
cookie_value="${cookie_left}${cookie_right}"

csrf_left='csrfValueAB'
csrf_right='CDEF1234+/='
csrf_value="${csrf_left}${csrf_right}"

token_left='tokenValueAB'
token_right='CDEF123456'
token_value="${token_left}${token_right}"

sign_left='signPayloadA'
sign_right='BCDEF12345'
sign_value="${sign_left}${sign_right}"

header_left='eyJhbGciOi'
header_right='JIUzI1NiJ9'
jwt_header="${header_left}${header_right}"

payload_left='eyJzdWIiOiJz'
payload_right='eW50aGV0aWMifQ'
jwt_payload="${payload_left}${payload_right}"

sig_left='c2lnbmF0dXJl'
sig_right='X3BheWxvYWQ'
jwt_signature="${sig_left}${sig_right}"
jwt_value="${jwt_header}.${jwt_payload}.${jwt_signature}"

phone_left='13800'
phone_right='138000'
phone_value="${phone_left}${phone_right}"

reset_repo_tree
write_repo_file backend/serialized.go <<EOF
package services

var syntheticCookieLine = "GAT=${cookie_value}; iget=${cookie_value}"
EOF
assert_rejected_repo serialized-cookie backend/serialized.go

reset_repo_tree
write_repo_file backend/cookie_assignment.go <<EOF
package services

func syntheticCookieAssignment() {
  csrfToken := "${csrf_value}"
}
EOF
assert_rejected_repo cookie-assignment backend/cookie_assignment.go

reset_repo_tree
write_repo_file backend/token_assignment.go <<EOF
package services

func syntheticTokenAssignments() {
  accessToken := "${token_value}"
  var accessToken = "${token_value}"
  _ = accessToken
}
EOF
assert_rejected_repo go-assign-token backend/token_assignment.go

reset_repo_tree
write_repo_file backend/jwt_assignment.go <<EOF
package services

func syntheticJwtAssignment() {
  jwtToken := "${jwt_value}"
  _ = jwtToken
}
EOF
assert_rejected_repo jwt-shape backend/jwt_assignment.go

reset_repo_tree
write_repo_file backend/sign_assignment.go <<EOF
package services

func syntheticSignAssignment() {
  var requestSign = "${sign_value}"
  _ = requestSign
}
EOF
assert_rejected_repo go-assign-sign backend/sign_assignment.go

reset_repo_tree
write_repo_file backend/mobile_literal.go <<EOF
package services

var syntheticPhone = "${phone_value}"
EOF
assert_rejected_repo quoted-mobile backend/mobile_literal.go

reset_repo_tree
write_repo_file frontend/src/assets/ignored.js <<EOF
const ignoredCookie = "GAT=${cookie_value}"
EOF
assert_clean_repo frontend-assets-exclusion
