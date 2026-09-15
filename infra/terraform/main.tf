terraform {
  required_version = ">= 1.6.0"
  required_providers {
    yandex = { source = "yandex-cloud/yandex", version = "~> 0.140" }
  }
}
provider "yandex" {
  cloud_id  = var.cloud_id
  folder_id = var.folder_id
  zone      = var.zone
  # Authentication: YC_TOKEN environment variable. Never commit tokens.
}
variable "cloud_id" { type = string }
variable "folder_id" { type = string }
variable "zone" { default = "ru-central1-a" }
variable "admin_cidr" {
  type        = string
  description = "Your public IP with /32, used for SSH"
}
variable "ssh_public_key_path" { type = string }
resource "yandex_vpc_network" "lab" { name = "music-school-lab2" }
resource "yandex_vpc_subnet" "lab" {
  name           = "music-school-lab2"
  zone           = var.zone
  network_id     = yandex_vpc_network.lab.id
  v4_cidr_blocks = ["10.20.0.0/24"]
}
resource "yandex_vpc_security_group" "lab" {
  name       = "music-school-lab2"
  network_id = yandex_vpc_network.lab.id
  ingress {
    protocol       = "TCP"
    port           = 22
    v4_cidr_blocks = [var.admin_cidr]
  }
  ingress {
    protocol       = "TCP"
    port           = 80
    v4_cidr_blocks = ["0.0.0.0/0"]
  }
  ingress {
    protocol       = "TCP"
    port           = 443
    v4_cidr_blocks = ["0.0.0.0/0"]
  }
  ingress {
    protocol          = "ANY"
    predefined_target = "self_security_group"
    from_port         = 0
    to_port           = 65535
  }
  egress {
    protocol       = "ANY"
    from_port      = 0
    to_port        = 65535
    v4_cidr_blocks = ["0.0.0.0/0"]
  }
}
data "yandex_compute_image" "ubuntu" { family = "ubuntu-2404-lts" }
resource "yandex_compute_instance" "lab" {
  name        = "music-school-lab2"
  platform_id = "standard-v3"
  resources {
    cores  = 2
    memory = 4
  }
  boot_disk {
    initialize_params {
      image_id = data.yandex_compute_image.ubuntu.id
      size     = 30
      type     = "network-ssd"
    }
  }
  network_interface {
    subnet_id          = yandex_vpc_subnet.lab.id
    nat                = true
    security_group_ids = [yandex_vpc_security_group.lab.id]
  }
  metadata = { ssh-keys = "ubuntu:${file(pathexpand(var.ssh_public_key_path))}" }
}
output "public_ip" { value = yandex_compute_instance.lab.network_interface[0].nat_ip_address }
output "ansible_inventory" { value = "[lab]\n${yandex_compute_instance.lab.network_interface[0].nat_ip_address} ansible_user=ubuntu\n" }
