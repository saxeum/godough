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
# REQUIRE_BROWSER=1 turns a missing headless browser into a failure instead of
# a skip, so CI cannot quietly drop the rendering half.
set -uo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
API_URL="${API_URL:-http://localhost:4000}"
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

seed_id() { # title
  body "/api/snippet" | python3 -c "
import json,sys
try:
    items = json.load(sys.stdin)
except Exception as exc:
    print('not json: %s' % exc, file=sys.stderr); raise SystemExit(1)
print(next((str(x['id']) for x in items if x['title'] == sys.argv[1]), ''))
" "$1"
}

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

# --- the API ---------------------------------------------------------------
expect_status "api: list" "/api/snippet" 200
ctype=$(content_type "/api/snippet")
[ "$ctype" = "application/json" ] && pass "api: list content-type" || fail "api: list content-type" "got $ctype"
# Not a count: this database is shared with a running UI, so other rows may
# exist. The three seed rows must be present, newest first among themselves.
SEEDS="('First autumn morning', 'Over the wintry forest', 'An old silent pond')"
expect_json "api: list has the seed snippets, newest first" "/api/snippet" \
  "[t for t in (x['title'] for x in d) if t in $SEEDS] == list($SEEDS)"
seed_pond=$(seed_id "An old silent pond")
if [ -z "$seed_pond" ]; then
  fail "api: seed snippet present" "no 'An old silent pond' in the list"
else
  expect_json "api: one snippet" "/api/snippet/$seed_pond" "d['id'] == $seed_pond and d['title'] == 'An old silent pond'"
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

  dom=$(render "/")
  if [[ "$dom" == *"Latest Snippets"* && "$dom" == *"First autumn morning"* && "$dom" == *"Over the wintry forest"* && "$dom" == *"An old silent pond"* ]]; then
    pass "render: list shows the three seed snippets"
  else
    fail "render: list shows the three seed snippets" "the rendered DOM is missing titles or the heading"
  fi

  if [ -n "$seed_pond" ]; then
    dom=$(render "/snippet/$seed_pond")
    if [[ "$dom" == *"An old silent pond..."* ]]; then
      pass "render: view shows the snippet"
    else
      fail "render: view shows the snippet" "the rendered DOM has no snippet body"
    fi
  else
    fail "render: view shows the snippet" "no seed id to render"
  fi

  dom=$(render "/snippet/abc")
  if [[ "$dom" == *"That snippet does not exist."* ]]; then
    pass "render: not-found state"
  else
    fail "render: not-found state" "the rendered DOM has no not-found message"
  fi
fi

# --- lifecycle: create, read, delete ---------------------------------------
# Unique title so a leftover row from a failed run is identifiable, and a trap
# so a failure after the create does not poison the next run's row count.
probe_id=""
probe_alive=0
cleanup_probe() {
  if [ "$probe_alive" = "1" ]; then
    curl -s -o /dev/null -X DELETE "$BASE_URL/api/snippet/$probe_id"
  fi
  true
}
trap cleanup_probe EXIT

resp=$(curl -s -X POST "$BASE_URL/api/snippet" -H 'Content-Type: application/json' \
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
