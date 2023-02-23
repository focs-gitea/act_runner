variable "registry" {
  default = "git.sogga.dev"
}

variable "namespace" {
  default = "pj/runner"
}

variable "VERSION" {
  default = "dev"
}

variable "platforms" {
  default = distinct(
    concat(
      windows(),
      darwin(),
      linux(),
    ),
  )
}

group "default" {
  targets = ["cross-build"]
}

target "base" {
  dockerfile = "Dockerfile"
  tags = [
    notequal("dev",VERSION) ? "${registry}/${namespace}:latest" : "",
    "${registry}/${namespace}:${VERSION}",
  ]
  args = {
    VERSION = VERSION
  }
}

target "cross-build" {
  platforms = platforms
  inherits = ["base"]
}

// ====================

function "windows" {
  params = []
  result = [
    "windows/386",
    "windows/amd64",
    "windows/arm64",
  ]
}

function "darwin" {
  params = []
  result = [
    "darwin/amd64",
    "darwin/arm64",
  ]
}

function "linux" {
  params = []
  result = [
    "linux/386",
    "linux/amd64",
    "linux/arm/v5",
    "linux/arm/v6",
    "linux/arm/v7",
    "linux/arm64/v8",
    "linux/s390x",
    "linux/ppc64le",
    "linux/mips64le",
    "linux/mips64",
    "linux/riscv64",
  ]
}
