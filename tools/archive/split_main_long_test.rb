#!/usr/bin/env ruby

content = File.read('main_long_test.go')
parts = content.split(/^func TestEmacsInSandbox/)

header = parts[0]
helpers = "func TestEmacsInSandbox" + parts[1]

File.write('main_long_test.go', header)
File.write('main_long_extra_test.go', "package main\n\nimport (\n\t\"context\"\n\t\"testing\"\n\t\"time\"\n)\n\n" + helpers)
