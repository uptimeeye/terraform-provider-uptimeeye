resource "uptimeeye_status_page_domain" "status" {
  status_page_id = uptimeeye_status_page.public.id
  hostname       = "status.example.com"
}
