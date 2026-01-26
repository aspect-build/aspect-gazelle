require 'io/console'

def main
  stdout = $stdout
  stdout.sync = true
  stdout.puts("Hello, world!") || raise("Failed to write to stdout")
end

main
