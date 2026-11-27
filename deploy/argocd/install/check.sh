#!/usr/bin/env bash
# Prove three things about the Argo CD install, because a values typo here is found
# by a controller that then cannot start, which reads like a broken cluster:
#
#   1. the reference is a content digest, not a mutable tag;
#   2. it renders with install/values.yaml;
#   3. the chart version it resolves to is the one infra/terraform/app pins, so the
#      digest line and the Terraform file cannot drift apart.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "${here}/../../.." && pwd)"
chart="$(tr -d '[:space:]' < "${here}/chart-ref.txt")"

printf '%s' "${chart}" | grep -Eq '@sha256:[0-9a-f]{64}$' || {
  echo "check.sh: ${chart} is not digest-pinned; expected oci://...@sha256:<64 hex>" >&2
  exit 1
}

# helm assumes Kubernetes 1.20 against an OCI chart with no cluster to ask, and the
# argo-cd chart refuses anything below 1.25 — so the version is pinned here rather
# than discovered.
kube_version="${KUBE_VERSION:-1.37.0}"
tmp="$(mktemp)"
trap 'rm -f "${tmp}"' EXIT

echo "check.sh: rendering $(basename "${chart}") (kube ${kube_version})"
helm template argocd "${chart}" \
  --namespace argocd \
  --kube-version "${kube_version}" \
  --include-crds \
  --values "${here}/values.yaml" > "${tmp}"

# A successful render is not the right chart: a digest that pointed elsewhere would
# still render. These are the objects the install is waited on.
for want in 'name: argocd-server' 'name: argocd-application-controller' \
            'name: applications.argoproj.io' 'name: applicationsets.argoproj.io'; do
  grep -q -- "${want}" "${tmp}" || {
    echo "check.sh: rendered chart has no '${want}' -- is the digest the argo-cd chart?" >&2
    exit 1
  }
done

rendered_version="$(grep -o 'helm.sh/chart: argo-cd-[0-9.]*' "${tmp}" | head -1 | sed 's/.*argo-cd-//')"
# The version inside the helm_release.argocd block, not any other `version =` in the
# file: there are several stacks' worth of resources in main.tf.
tf_version="$(awk '/resource "helm_release" "argocd"/,/^}/' "${root}/infra/terraform/app/main.tf" \
  | grep -oP 'version\s*=\s*"\K[^"]+' | head -1)"

echo "check.sh: digest renders argo-cd ${rendered_version}; terraform pins ${tf_version}"
[ "${rendered_version}" = "${tf_version}" ] || {
  echo "check.sh: the digest in chart-ref.txt resolves to argo-cd ${rendered_version}, but infra/terraform/app installs ${tf_version}" >&2
  echo "check.sh: update chart-ref.txt (and re-run) or the version in main.tf" >&2
  exit 1
}

echo "check.sh: pinned, renders, and agrees with Terraform"
