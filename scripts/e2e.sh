#!/usr/bin/env bash
#
# End-to-end checks against a running stack. Nothing here starts or stops a
# container: it asserts what a browser would see at the LB.
#
#   BASE_URL=http://localhost:8080 scripts/e2e.sh
#
# REQUIRE_BROWSER=1 turns a missing headless browser into a failure instead of
# a skip, so CI cannot quietly drop the rendering half.
set -uo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
REQUIRE_BROWSER="${REQUIRE_BROWSER:-0}"
fails=0
checks=0

pass() { checks=$((checks + 1)); printf 'ok    %s\n' "$1"; }
fail() { checks=$((checks + 1)); fails=$((fails + 1)); printf 'FAIL  %s\n      %s\n' "$1" "$2"; }

status() { curl -s -o /dev/null -w '%{http_code}' "$BASE_URL$1"; }
content_type() { curl -s -o /dev/null -w '%{content_type}' "$BASE_URL$1"; }
body() { curl -s "$BASE_URL$1"; }

expect_status() { # name path want [method]
  local got
  got=$(curl -s -o /dev/null -w '%{http_code}' -X "${4:-GET}" "$BASE_URL$2")
  if [ "$got" = "$3" ]; then pass "$1 ($2 -> $got)"; else fail "$1 ($2 ${4:-GET})" "status $got, want $3"; fi
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

echo "# $BASE_URL"

# --- the API ---------------------------------------------------------------
expect_status "api: list" "/api/snippets" 200
ctype=$(content_type "/api/snippets")
[ "$ctype" = "application/json" ] && pass "api: list content-type" || fail "api: list content-type" "got $ctype"
expect_json "api: list is 3 snippets, newest first" "/api/snippets" \
  "len(d) == 3 and d[0]['Title'] == 'First autumn morning' and d[-1]['Title'] == 'An old silent pond'"
expect_json "api: one snippet" "/api/snippet/view/1" "d['ID'] == 1 and d['Title'] == 'An old silent pond'"
expect_status "api: malformed id is 404" "/api/snippet/view/abc" 404
expect_json "api: malformed id is a JSON error" "/api/snippet/view/abc" "d.get('error') == 'Not Found'"
expect_status "api: missing id is 404" "/api/snippet/view/999" 404
expect_status "api: wrong method is 405" "/api/snippet/view/1" 405 POST

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

  dom=$(render "/")
  if [[ "$dom" == *"Latest Snippets"* && "$dom" == *"First autumn morning"* && "$dom" == *"Over the wintry forest"* && "$dom" == *"An old silent pond"* ]]; then
    pass "render: list shows the three seed snippets"
  else
    fail "render: list shows the three seed snippets" "the rendered DOM is missing titles or the heading"
  fi

  dom=$(render "/snippet/1")
  if [[ "$dom" == *"An old silent pond..."* ]]; then
    pass "render: view shows the snippet"
  else
    fail "render: view shows the snippet" "the rendered DOM has no snippet body"
  fi

  dom=$(render "/snippet/abc")
  if [[ "$dom" == *"That snippet does not exist."* ]]; then
    pass "render: not-found state"
  else
    fail "render: not-found state" "the rendered DOM has no not-found message"
  fi
fi

echo
if [ "$fails" -eq 0 ]; then
  echo "$checks checks, 0 failed"
  exit 0
fi
echo "$checks checks, $fails failed"
exit 1
