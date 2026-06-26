provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

# ── Static external IP ───────────────────────────────────────────────────────

resource "google_compute_address" "app_ip" {
  name   = "aios-ip"
  region = var.region
}

# ── Firewall ─────────────────────────────────────────────────────────────────
# Allows inbound traffic on the ports the stack exposes.
# All services run on a single VM so one rule covers everything.

resource "google_compute_firewall" "aios_inbound" {
  name    = "aios-inbound"
  network = "default"

  allow {
    protocol = "tcp"
    ports    = [
      "22",   # SSH
      "80",   # web (nginx)
      "8080", # api (Go)
      "8000", # agent (Python)
      "8180", # camunda7
    ]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["aios"]
}

# ── VM ───────────────────────────────────────────────────────────────────────

resource "google_compute_instance" "app" {
  name         = "aios-vm"
  machine_type = var.machine_type
  zone         = var.zone
  tags         = ["aios"]

  boot_disk {
    initialize_params {
      image = "ubuntu-os-cloud/ubuntu-2204-lts"
      size  = 50   # GB — enough for Docker images + Postgres data
      type  = "pd-balanced"
    }
  }

  network_interface {
    network = "default"
    access_config {
      nat_ip = google_compute_address.app_ip.address
    }
  }

  # Injects your public key so Ansible can SSH in immediately after apply.
  metadata = {
    ssh-keys = "${var.ssh_user}:${file(var.ssh_pub_key_path)}"
  }

  lifecycle {
    # Prevent Terraform from re-creating the VM if the key changes locally.
    ignore_changes = [metadata]
  }
}
