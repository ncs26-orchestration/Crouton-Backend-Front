output "vm_ip" {
  description = "External IP of the VM — paste this into infra/ansible/inventory.ini"
  value       = google_compute_address.app_ip.address
}

output "vm_name" {
  description = "VM instance name"
  value       = google_compute_instance.app.name
}

output "ssh_command" {
  description = "Ready-to-use SSH command"
  value       = "ssh ${var.ssh_user}@${google_compute_address.app_ip.address}"
}
