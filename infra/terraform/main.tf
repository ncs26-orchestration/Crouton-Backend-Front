provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

# ── Firewall ─────────────────────────────────────────────────────────────────
# Adds the app ports on top of the project's existing default rules (which
# already allow 22 and 80). Targets the VM's existing "http-server" tag.

resource "google_compute_firewall" "aios_inbound" {
  name    = "aios-inbound"
  network = "default"

  allow {
    protocol = "tcp"
    ports = [
      "80",   # web (nginx) — also covered by allow-http, kept for clarity
      "443",  # HTTPS (Caddy reverse proxy, Let's Encrypt)
      "8080", # api (Go)
      "8000", # agent (Python)
      "8180", # camunda7 cockpit (demo)
    ]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["http-server"]
}

# ── VM ───────────────────────────────────────────────────────────────────────
# Describes the pre-existing "ncs26-vm" so Terraform can adopt it via
# `terraform import` without rebuilding it. Attributes mirror the live instance;
# changing an immutable one (name, zone, image, disk) would force a replace.

resource "google_compute_instance" "app" {
  name         = "ncs26-vm"
  machine_type = var.machine_type
  zone         = var.zone
  tags         = ["http-server"]

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-12"
      size  = 30
      type  = "pd-standard"
    }
  }

  network_interface {
    network = "default"
    # Empty access_config = ephemeral external IP (no reserved address).
    access_config {}
  }

  service_account {
    email  = var.service_account_email
    scopes = ["cloud-platform"]
  }

  lifecycle {
    # The VM carries a startup-script and OS Login keys we don't manage here, and
    # the Debian boot image patch level drifts — ignore both so day-to-day plans
    # stay clean and never propose a rebuild.
    ignore_changes = [
      metadata,
      boot_disk[0].initialize_params[0].image,
    ]
  }
}
