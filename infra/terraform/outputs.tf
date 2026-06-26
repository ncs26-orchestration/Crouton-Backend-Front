output "vm_ip" {
  description = "External IP of the VM — paste this into infra/ansible/inventory.ini"
  value       = google_compute_instance.app.network_interface[0].access_config[0].nat_ip
}

output "vm_name" {
  description = "VM instance name"
  value       = google_compute_instance.app.name
}

output "ssh_command" {
  description = "Ready-to-use SSH command (OS Login)"
  value       = "gcloud compute ssh ${google_compute_instance.app.name} --zone ${var.zone}"
}
