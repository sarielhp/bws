#!/usr/bin/env ruby

content = File.read('commands.go')
parts = content.split(/^func addCmd/)

header = parts[0]
helpers = "func addCmd" + parts[1]

File.write('commands.go', header)
File.write('commands_mod.go', "package main\n\nimport (\n\t\"github.com/sarielhp/clihelp\"\n\t\"bws/internal/cli\"\n)\n\n" + helpers)
