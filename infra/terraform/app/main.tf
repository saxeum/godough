locals {
  charts = "${path.module}/../../../deploy/charts"

  api_values = {
    image = {
      repository = "${var.image_registry}/${var.project}-api"
      digest     = var.api_digest
    }
    trustedOrigins = var.trusted_origins
    # The Secrets are created by `make rds-app-password` and the sync that #125
    # decides on; the chart only names them.
    databaseSecret = { name = "godough-api", key = "DATABASE_URL" }
    caSecret       = { name = "godough-rds-ca", key = "global-bundle.pem" }
  }

  ui_values = {
    image = {
      repository = "${var.image_registry}/${var.project}-ui"
      digest     = var.ui_digest
    }
  }
}

resource "helm_release" "api" {
  name      = "godough-api"
  chart     = "${local.charts}/api"
  namespace = var.namespace
  # Created here rather than by a second provider for two workloads.
  create_namespace = true

  values = [yamlencode(local.api_values)]

  # Applied means running, not merely applied. Without this a rollout that
  # cannot start (a missing Secret, an unpullable digest) reports success.
  wait    = true
  timeout = 300
}

resource "helm_release" "ui" {
  name             = "godough-ui"
  chart            = "${local.charts}/ui"
  namespace        = var.namespace
  create_namespace = false

  values = [yamlencode(local.ui_values)]

  wait    = true
  timeout = 300

  depends_on = [helm_release.api]
}
