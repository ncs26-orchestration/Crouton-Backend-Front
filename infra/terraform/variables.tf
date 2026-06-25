variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
  default     = "us-central1"
}

variable "zone" {
  description = "GCP zone"
  type        = string
  default     = "us-central1-a"
}

variable "machine_type" {
  description = "VM machine type"
  type        = string
  default     = "e2-standard-2"
}

variable "ssh_user" {
  description = "OS login username for SSH"
  type        = string
  default     = "deploy"
}

variable "ssh_pub_key_path" {
  description = "Path to the SSH public key to inject into the VM"
  type        = string
  default     = "~/.ssh/id_rsa.pub"
}
