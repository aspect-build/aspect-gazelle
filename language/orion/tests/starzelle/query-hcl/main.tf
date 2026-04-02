variable "name" {
  type    = string
  default = "world"
}

resource "local_file" "greeting" {
  content  = "Hello, ${var.name}!"
  filename = "/tmp/greeting.txt"
}
