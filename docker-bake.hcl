variable "VERSION" {
  default = "latest"
}

variable "IMAGE" {
  default = "ghcr.io/acme/controller"
}

group "default" {
  targets = ["image", "debug"]
}

target "docker-metadata-action" {}
target "docker-metadata-action-debug" {}

target "image" {
  inherits = ["docker-metadata-action"]

  tags = [
    "${IMAGE}:${VERSION}",
    "${IMAGE}:latest",
  ]

  platforms = [
    "linux/amd64",
    "linux/arm64",
  ]
}

target "debug" {
  inherits = ["docker-metadata-action-debug"]

  target = "debug"

  tags = [
    "${IMAGE}:${VERSION}-debug",
    "${IMAGE}:debug",
  ]

  platforms = [
    "linux/amd64",
    "linux/arm64",
  ]
}