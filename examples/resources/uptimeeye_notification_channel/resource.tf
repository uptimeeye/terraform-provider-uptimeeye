resource "uptimeeye_notification_channel" "oncall" {
  name = "On-Call"
  tags = ["managed-by:terraform"]
}
