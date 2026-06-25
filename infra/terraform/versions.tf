terraform {
  required_version = ">= 1.7"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }

  # Uncomment to store state in GCS (recommended before sharing with team):
  # backend "gcs" {
  #   bucket = "YOUR_GCS_BUCKET"
  #   prefix = "aup/terraform"
  # }
}
