#!/usr/bin/env bash
#
# End-to-end checks against a running stack. Nothing here starts or stops a
# container: it asserts what a browser would see at the LB, and it checks the
# API's probes on the API's own port.
#
#   BASE_URL=http://localhost:8080 API_URL=http://localhost:4000 scripts/e2e.sh
#
# The probes are deliberately not part of the public route table. In production
# the ALB target group checks the api pod directly; the LB and the ingress never
# route /healthz or /readyz.
#
# The snippet routes require a session. This script creates one directly in the
# local database unless E2E_SESSION is set; there is no sign-in endpoint yet.
#
# REQUIRE_BROWSER=1 turns a missing headless browser into a failure instead of
# a skip, so CI cannot quietly drop the rendering half.
set -uo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
API_URL="${API_URL:-http://localhost:4000}"
REQUIRE_BROWSER="${REQUIRE_BROWSER:-0}"
SESSION="${E2E_SESSION:-}"
DB_USER_SUB=""
probe_id=""
probe_alive=0
fails=0
checks=0
AUTH=()

pass() { checks=$((checks + 1)); printf 'ok    %s\n' "$1"; }
fail() { checks=$((checks + 1)); fails=$((fails + 1)); printf 'FAIL  %s\n      %s\n' "$1" "$2"; }

cleanup() {
  if [ "$probe_alive" = "1" ]; then
    curl -s -o /dev/null -X DELETE -H "Origin: $BASE_URL" "${AUTH[@]}" "$BASE_URL/api/snippet/$probe_id"
  fi
  if [ -n "$DB_USER_SUB" ]; then
    docker compose exec -T postgres psql -q -c "delete from users where sub = '$DB_USER_SUB'" -U godough -d snippet >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

status() { curl -s -o /dev/null -w '%{http_code}' "${AUTH[@]}" "$BASE_URL$1"; }
content_type() { curl -s -o /dev/null -w '%{content_type}' "${AUTH[@]}" "$BASE_URL$1"; }
body() { curl -s "${AUTH[@]}" "$BASE_URL$1"; }

expect_status() { # name path want [method]
  local method="${4:-GET}" got extra=()
  case "$method" in
    POST|PUT|PATCH|DELETE) extra=(-H "Origin: $BASE_URL") ;;
  esac
  got=$(curl -s -o /dev/null -w '%{http_code}' "${AUTH[@]}" "${extra[@]}" -X "$method" "$BASE_URL$2")
  if [ "$got" = "$3" ]; then pass "$1 ($2 -> $got)"; else fail "$1 ($2 $method)" "status $got, want $3"; fi
}

expect_api_body() { # name path want
  local got
  got=$(curl -s "$API_URL$2")
  if [ "$got" = "$3" ]; then pass "$1 ($API_URL$2)"; else fail "$1 ($API_URL$2)" "body $(printf '%q' "$got"), want $3"; fi
}

expect_json() { # name path python-expression-on-d
  local got
  got=$(body "$2" | python3 -c "
import json,sys
try:
    d = json.load(sys.stdin)
except Exception as exc:
    print('not json: %s' % exc); raise SystemExit(1)
print($3)
" 2>&1) || { fail "$1 ($2)" "$got"; return; }
  if [ "$got" = "True" ]; then pass "$1 ($2)"; else fail "$1 ($2)" "got: $got"; fi
}

create_snippet() { # title -> id, with the test session and a trusted Origin
  local resp
  resp=$(curl -s -X POST "$BASE_URL/api/snippet" -H 'Content-Type: application/json' \
    -H "Origin: $BASE_URL" "${AUTH[@]}" \
    -d "{\"title\":\"$1\",\"content\":\"created by scripts/e2e.sh\",\"expiresInDays\":1}")
  printf '%s' "$resp" | python3 -c 'import json,sys
try: print(json.load(sys.stdin).get("id") or "")
except Exception: print("")'
}

# There is no sign-in endpoint yet (#53), so the e2e session is written straight
# into the local database. The hash is the same SHA-256 hex the service stores.
if [ -z "$SESSION" ]; then
  read -r SESSION SESSION_HASH <<<"$(python3 -c 'import hashlib,secrets; t=secrets.token_urlsafe(32); print(t, hashlib.sha256(t.encode()).hexdigest())')"
  DB_USER_SUB="e2e-$$"
  user_id=$(docker compose exec -T postgres psql -q -tAc "insert into users (sub, email, name) values ('$DB_USER_SUB', 'e2e@example.com', 'e2e') on conflict (sub) do update set email = excluded.email returning id" -U godough -d snippet 2>/dev/null | tr -d '[:space:]')
  if [ -n "$user_id" ]; then
    docker compose exec -T postgres psql -q -v ON_ERROR_STOP=1 -c "insert into sessions (token_hash, user_id, expires, ip, user_agent) values ('$SESSION_HASH', $user_id, now() + interval '1 hour', '127.0.0.1', 'scripts/e2e.sh')" -U godough -d snippet >/dev/null 2>&1 || SESSION=""
  else
    SESSION=""
  fi
  if [ -z "$SESSION" ]; then
    fail "e2e: session" "could not create a session in the local database"
  else
    AUTH=(-b "session=$SESSION")
  fi
fi
if [ -n "$SESSION" ] && [ "${#AUTH[@]}" -eq 0 ]; then
  AUTH=(-b "session=$SESSION")
fi

echo "# $BASE_URL"
echo "# api $API_URL"

# --- the probes: pod-level, not public -------------------------------------
# Direct to the API, because that is what the ALB target group and the kubelet
# do. The LB assertions below prove the web fallback is not standing in for a
# probe: a 200 there would be a health check that passes while the API is dead.
expect_api_body "api: healthz direct" "/healthz" "ok"
expect_api_body "api: readyz direct" "/readyz" "ok"
expect_status "lb: /healthz is not a client route" "/healthz" 404
expect_status "lb: /readyz is not a client route" "/readyz" 404

# --- the session and CSRF boundary -----------------------------------------
anonymous=$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/api/snippet")
if [ "$anonymous" = "401" ]; then pass "api: list without a session is 401"; else fail "api: list without a session is 401" "status $anonymous"; fi

anonymous_write=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE_URL/api/snippet" \
  -H 'Content-Type: application/json' -H 'Origin: https://evil.example' \
  -d '{"title":"anonymous","content":"x","expiresInDays":1}')
if [ "$anonymous_write" = "401" ]; then pass "api: anonymous write is 401 before CSRF"; else fail "api: anonymous write is 401 before CSRF" "status $anonymous_write"; fi

if [ "${#AUTH[@]}" -gt 0 ]; then
  no_origin=$(curl -s -o /dev/null -w '%{http_code}' "${AUTH[@]}" -X POST "$BASE_URL/api/snippet" \
    -H 'Content-Type: application/json' -d '{"title":"csrf-blocked","content":"x","expiresInDays":1}')
  if [ "$no_origin" = "403" ]; then pass "api: write without Origin is 403"; else fail "api: write without Origin is 403" "status $no_origin"; fi

  bad_origin=$(curl -s -o /dev/null -w '%{http_code}' "${AUTH[@]}" -X POST "$BASE_URL/api/snippet" \
    -H 'Content-Type: application/json' -H 'Origin: https://evil.example' \
    -d '{"title":"csrf-evil","content":"x","expiresInDays":1}')
  if [ "$bad_origin" = "403" ]; then pass "api: write with a foreign Origin is 403"; else fail "api: write with a foreign Origin is 403" "status $bad_origin"; fi

  expect_json "api: rejected CSRF write created no row" "/api/snippet" \
    "not any(x['title'] in ('csrf-blocked', 'csrf-evil') for x in d)"

  set_cookie=$(curl -s -D - -o /dev/null "${AUTH[@]}" "$BASE_URL/api/snippet" | grep -ci 'set-cookie')
  if [ "$set_cookie" = "0" ]; then pass "api: a valid session does not churn the cookie"; else fail "api: a valid session does not churn the cookie" "$set_cookie Set-Cookie headers"; fi
fi

# --- the API ---------------------------------------------------------------
first_id=$(create_snippet "e2e first")
second_id=$(create_snippet "e2e second")
if [ -z "$first_id" ] || [ -z "$second_id" ]; then
  fail "api: create the e2e rows" "ids came back as '$first_id' and '$second_id'"
fi

expect_status "api: list" "/api/snippet" 200
ctype=$(content_type "/api/snippet")
[ "$ctype" = "application/json" ] && pass "api: list content-type" || fail "api: list content-type" "got $ctype"
# The list is scoped to this session's owner, and the two rows just created are
# the only ones this new user can see. The second create is newer, so it leads.
expect_json "api: list is owner-scoped and newest first" "/api/snippet" \
  "[t for t in (x['title'] for x in d) if t in ('e2e first', 'e2e second')] == ['e2e second', 'e2e first']"
if [ -n "$first_id" ]; then
  expect_json "api: one snippet" "/api/snippet/$first_id" "d['id'] == $first_id and d['title'] == 'e2e first'"
fi
# Not an integer at all: the router rejects it before any handler runs.
expect_status "api: malformed id is 400" "/api/snippet/abc" 400
expect_json "api: malformed id is a JSON error" "/api/snippet/abc" "d.get('error') == 'Bad Request'"
expect_status "api: missing id is 404" "/api/snippet/999" 404
expect_status "api: wrong method is 405" "/api/snippet/999" 405 POST

# --- the web container behind the LB ---------------------------------------
expect_status "web: index" "/" 200
expect_status "web: SPA fallback for a client route" "/nope" 200
b=$(body "/")
[[ "$b" == *'id="root"'* ]] && pass "web: index mounts the app" || fail "web: index mounts the app" "no #root in the body"

asset=$(grep -oE 'assets/index-[A-Za-z0-9_-]+\.js' <<<"$b" | head -1)
if [ -z "$asset" ]; then
  fail "web: hashed asset" "index.html names no assets/index-*.js"
else
  expect_status "web: hashed asset" "/$asset" 200
  cc=$(curl -s -o /dev/null -D - "$BASE_URL/$asset" | tr -d '\r' | awk -F': ' 'tolower($1)=="cache-control"{print $2}')
  [[ "$cc" == *immutable* ]] && pass "web: hashed asset is cached immutably" || fail "web: hashed asset is cached immutably" "Cache-Control: ${cc:-absent}"
fi

# --- the rendered app ------------------------------------------------------
browser=""
if [ -n "${CHROME_BIN:-}" ] && command -v "${CHROME_BIN}" >/dev/null; then
  browser="${CHROME_BIN}"
else
  for c in chromium chromium-browser google-chrome google-chrome-stable; do
    command -v "$c" >/dev/null && { browser="$c"; break; }
  done
fi

if [ -z "$browser" ]; then
  if [ "$REQUIRE_BROWSER" = "1" ]; then
    fail "render: headless browser" "REQUIRE_BROWSER=1 but none of chromium/google-chrome was found"
  else
    echo "skip  render: no headless browser found, set CHROME_BIN or REQUIRE_BROWSER=0"
  fi
else
  render() { "$browser" --headless --no-sandbox --disable-gpu --virtual-time-budget=15000 --dump-dom "$BASE_URL$1" 2>/dev/null; }

  # The browser has no session cookie yet. The signed-in list/view checks come
  # back with the sign-in UI (#56); this proves the page fails closed.
  dom=$(render "/")
  if [[ "$dom" == *"Sign in required."* ]]; then
    pass "render: signed-out state on the list"
  else
    fail "render: signed-out state on the list" "the rendered DOM has no signed-out message"
  fi

  dom=$(render "/snippet/1")
  if [[ "$dom" == *"Sign in required."* ]]; then
    pass "render: signed-out state on a snippet"
  else
    fail "render: signed-out state on a snippet" "the rendered DOM has no signed-out message"
  fi

  dom=$(render "/nope")
  if [[ "$dom" == *"Page not found."* ]]; then
    pass "render: client route not-found state"
  else
    fail "render: client route not-found state" "the rendered DOM has no not-found message"
  fi
fi

# --- lifecycle: create, read, delete ---------------------------------------
# Unique title so a leftover row from a failed run is identifiable.
resp=$(curl -s -X POST "$BASE_URL/api/snippet" -H 'Content-Type: application/json' \
  -H "Origin: $BASE_URL" "${AUTH[@]}" \
  -d "{\"title\":\"e2e probe $$\",\"content\":\"created by scripts/e2e.sh\",\"expiresInDays\":1}")
probe_id=$(printf '%s' "$resp" | python3 -c 'import json,sys
try: print(json.load(sys.stdin).get("id") or "")
except Exception: print("")' 2>/dev/null)

if [ -n "$probe_id" ]; then
  probe_alive=1
  pass "api: create a snippet (id $probe_id)"
  expect_status "api: the created snippet is readable" "/api/snippet/$probe_id" 200
  expect_status "api: delete the snippet" "/api/snippet/$probe_id" 204 DELETE
  probe_alive=0
  expect_status "api: the deleted snippet is gone" "/api/snippet/$probe_id" 404
  expect_json "api: the probe is gone from the list" "/api/snippet" \
    "not any(x['title'] == 'e2e probe $$' for x in d)"
else
  fail "api: create a snippet" "no ID came back: $resp"
fi

echo
if [ "$fails" -eq 0 ]; then
  echo "$checks checks, 0 failed"
  exit 0
fi
echo "$checks checks, $fails failed"
exit 1
