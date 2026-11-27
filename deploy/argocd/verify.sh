#!/usr/bin/env bash
# What the cluster is, against what the repository says.
#
# Two questions, because either alone is a green light that lies:
#
#   1. every Application is Synced and Healthy — Argo agrees its desired state is
#      what is running;
#   2. for every chart-backed Application, the digest in the values file is the
#      digest on the running pod — the file, not just Argo's opinion of the file.
#
# A pod can be Synced/Healthy while running an image nobody chose: Argo says
# `Synced` about the objects it manages, and an image that was hand-patched onto a
# Deployment is drift it will happily revert *later*. The digest check is what
# catches the gap between the two.
#
# usage: verify.sh   (needs `make eks-tunnel`)
set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "${here}/../.." && pwd)"
checked=0
failed=0

fail() { failed=$((failed + 1)); printf 'FAIL  %s\n      %s\n' "$1" "$2"; }
pass() { checked=$((checked + 1)); printf 'ok    %s\n' "$1"; }

rows="$(kubectl -n argocd get applications \
  -o jsonpath='{range .items[*]}{.metadata.name}{"|"}{.status.sync.status}{"|"}{.status.health.status}{"|"}{.spec.source.path}{"|"}{.spec.destination.namespace}{"\n"}{end}')" || exit 1

if [ -z "${rows}" ]; then
  echo "verify: no Applications in the argocd namespace" >&2
  exit 1
fi

while IFS='|' read -r name sync health path namespace; do
  [ -n "${name}" ] || continue

  if [ "${sync}" != "Synced" ] || [ "${health}" != "Healthy" ]; then
    fail "${name}" "sync=${sync:-none} health=${health:-none}"
    continue
  fi
  pass "${name} is Synced and Healthy"

  case "${path}" in
    deploy/charts/*) ;;
    *) continue ;;   # the root syncs definitions; it has no workload to compare
  esac

  chart="${path#deploy/charts/}"
  values="${root}/${path}/values-staging.yaml"
  if [ ! -f "${values}" ]; then
    fail "${name}" "no values file at ${path}/values-staging.yaml"
    continue
  fi

  # The digest as the repository states it. Anchored to the image block so a
  # `digest:` anywhere else in the file cannot satisfy this.
  want="$(awk '/^image:/{inimage=1; next} inimage && /^[^ ]/{inimage=0} inimage && $1=="digest:"{gsub(/"/,"",$2); print $2; exit}' "${values}")"
  if [ -z "${want}" ]; then
    fail "${name}" "no image.digest in ${path}/values-staging.yaml"
    continue
  fi

  got="$(kubectl -n "${namespace}" get pods -l "app.kubernetes.io/instance=${name}" \
    -o jsonpath='{range .items[*]}{.spec.containers[0].image}{"\n"}{end}' 2>/dev/null)"
  if [ -z "${got}" ]; then
    fail "${name}" "no pods labelled app.kubernetes.io/instance=${name} in ${namespace}"
    continue
  fi

  # Every replica, not the first: a half-finished rollout runs two digests at once
  # and "the first pod matched" is exactly how that goes unnoticed.
  while read -r image; do
    [ -n "${image}" ] || continue
    case "${image}" in
      *"@${want}") ;;
      *) fail "${name}" "running ${image##*/}, values file says ${want}" ;;
    esac
  done <<< "${got}"

  case "${got}" in
    *"@${want}"*) pass "${name} runs ${want} on every replica" ;;
  esac
done <<< "${rows}"

echo
if [ "${failed}" -gt 0 ]; then
  echo "verify: ${failed} failure(s), ${checked} check(s) passed" >&2
  exit 1
fi
echo "verify: ${checked} checks passed; the cluster is what the repository says"
