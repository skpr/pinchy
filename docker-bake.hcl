variable "REGISTRY" {
  default = "ghcr.io"
}

variable "REPO" {
  default = "skpr/pinchy"
}

// Comma-separated bare tag suffixes.  CI overrides with e.g. "latest,sha-abc1234".
variable "TAGS" {
  default = "latest"
}

variable "PLATFORMS" {
  default = ["linux/amd64", "linux/arm64"]
}

// Returns the full list of image:tag strings for a given component prefix.
function "image" {
  params = [prefix]
  result = [for t in split(",", TAGS) : "${REGISTRY}/${REPO}:${prefix}-${t}"]
}

group "default" {
  targets = ["api", "operator", "proxy", "board", "opencode", "litellm"]
}

target "api" {
  context    = "."
  dockerfile = "images/api/Dockerfile"
  platforms  = PLATFORMS
  tags       = image("api")
}

target "operator" {
  context    = "."
  dockerfile = "images/operator/Dockerfile"
  platforms  = PLATFORMS
  tags       = image("operator")
}

target "proxy" {
  context    = "."
  dockerfile = "images/proxy/Dockerfile"
  platforms  = PLATFORMS
  tags       = image("proxy")
}

target "board" {
  context    = "."
  dockerfile = "images/board/Dockerfile"
  platforms  = PLATFORMS
  tags       = image("board")
}

target "opencode" {
  context    = "."
  dockerfile = "images/opencode/Dockerfile"
  platforms  = PLATFORMS
  tags       = image("opencode")
}

// litellm uses its own subdirectory as context because the Dockerfile
// copies config.yml relative to that directory.
target "litellm" {
  context    = "images/litellm"
  dockerfile = "Dockerfile"
  platforms  = PLATFORMS
  tags       = image("litellm")
}
