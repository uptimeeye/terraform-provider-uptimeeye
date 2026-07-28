# Cron-based heartbeat: alerts when the job does not check in on schedule.
resource "uptimeeye_scheduled_task" "nightly_backup" {
  name                    = "Nightly backup"
  notification_channel_id = uptimeeye_notification_channel.oncall.id
  grace_period            = 30

  cron_task = {
    cron_expression = "0 3 * * *"
    timezone        = "Europe/Berlin"
  }
}

# Interval-based heartbeat.
resource "uptimeeye_scheduled_task" "queue_worker" {
  name                    = "Queue worker"
  notification_channel_id = uptimeeye_notification_channel.oncall.id
  grace_period            = 5

  simple_task = {
    interval_type  = "minutes"
    interval_value = 15
  }
}
