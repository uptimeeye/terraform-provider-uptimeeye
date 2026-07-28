terraform {
  required_providers {
    uptimeeye = {
      source  = "uptimeeye/uptimeeye"
      version = ">= 0.1.0"
    }
  }
}

variable "uptimeeye_api_key" {
  type      = string
  sensitive = true
}

variable "slack_webhook_url" {
  type      = string
  sensitive = true
}

provider "uptimeeye" {
  api_key = var.uptimeeye_api_key
}

# --- Alerting ---------------------------------------------------------------

resource "uptimeeye_notification_channel" "oncall" {
  name = "On-Call"
  tags = ["managed-by:terraform"]
}

resource "uptimeeye_notification_integration" "slack" {
  channel_id = uptimeeye_notification_channel.oncall.id
  name       = "Slack #alerts"

  slack = {
    webhook_url = var.slack_webhook_url
    channel     = "#alerts"
  }
}

resource "uptimeeye_notification_integration" "email" {
  channel_id = uptimeeye_notification_channel.oncall.id
  name       = "Ops mailing list"

  email = {
    to = ["ops@example.com"]
  }
}

# --- Monitoring -------------------------------------------------------------

data "uptimeeye_locations" "all" {}

resource "uptimeeye_variable" "api_token" {
  key    = "API_TOKEN"
  value  = "replace-me"
  secure = true
}

resource "uptimeeye_monitor" "api" {
  name                    = "API Healthcheck"
  notification_channel_id = uptimeeye_notification_channel.oncall.id
  locations               = ["eu-central-1"]
  tags                    = ["env:prod", "managed-by:terraform"]

  steps = [{
    name = "GET /health"
    type = "http"
    request = {
      url    = "https://api.example.com/health"
      method = "GET"
      header = [{
        key   = "Authorization"
        value = "Bearer {{API_TOKEN}}"
      }]
    }
    asserts = [{
      type     = "body"
      property = "statusCode"
      operator = "is"
      expected = "200"
    }]
  }]

  options = {
    tick_every        = 60
    timeout           = 5000
    failure_threshold = 2
  }
}

resource "uptimeeye_scheduled_task" "nightly_backup" {
  name                    = "Nightly backup"
  notification_channel_id = uptimeeye_notification_channel.oncall.id
  grace_period            = 30

  cron_task = {
    cron_expression = "0 3 * * *"
    timezone        = "Europe/Berlin"
  }
}

# --- Status page ------------------------------------------------------------

resource "uptimeeye_status_page" "public" {
  name  = "Example Status"
  slug  = "example-status"
  theme = "auto"

  sections = [{
    name        = "API"
    description = "Core API services"
    monitors = [{
      monitor_id  = uptimeeye_monitor.api.id
      show_uptime = true
    }]
    scheduled_tasks = [{
      scheduled_task_id = uptimeeye_scheduled_task.nightly_backup.id
    }]
  }]
}

resource "uptimeeye_report_schedule" "weekly" {
  status_page_id = uptimeeye_status_page.public.id
  cadence        = "weekly"
  recipients     = ["reports@example.com"]
}
