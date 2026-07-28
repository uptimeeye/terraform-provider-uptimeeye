resource "uptimeeye_report_schedule" "weekly" {
  status_page_id = uptimeeye_status_page.public.id
  cadence        = "weekly"
  recipients     = ["reports@example.com"]
}
