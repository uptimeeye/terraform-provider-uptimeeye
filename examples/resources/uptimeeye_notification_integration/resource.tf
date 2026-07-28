resource "uptimeeye_notification_integration" "slack" {
  channel_id = uptimeeye_notification_channel.oncall.id
  name       = "Slack #alerts"

  slack = {
    webhook_url = var.slack_webhook_url
    channel     = "#alerts"
  }
}

resource "uptimeeye_notification_integration" "pagerduty" {
  channel_id = uptimeeye_notification_channel.oncall.id
  name       = "PagerDuty"

  pagerduty = {
    routing_key = var.pagerduty_routing_key
  }
}
