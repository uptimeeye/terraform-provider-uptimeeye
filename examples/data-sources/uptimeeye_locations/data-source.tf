data "uptimeeye_locations" "all" {}

output "locations" {
  value = data.uptimeeye_locations.all.locations
}
