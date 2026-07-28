resource "uptimeeye_status_page" "public" {
  name      = "Example Status"
  slug      = "example-status"
  is_public = true
  theme     = "auto"

  sections = [{
    name = "API"
    monitors = [{
      monitor_id = uptimeeye_monitor.api.id
    }]
  }]
}
