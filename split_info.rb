#!/usr/bin/env ruby

content = File.read('internal/cli/info.go')
parts = content.split(/^func PrintInfo/)

header = parts[0]
print_info_func = "func PrintInfo" + parts[1]

# Write header back to info.go (with imports adjusted later by goimports/gofmt)
File.write('internal/cli/info.go', header)

# Write PrintInfo and below to info_print.go
File.write('internal/cli/info_print.go', "package cli\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"strings\"\n\n\t\"bws/internal/config\"\n\t\"bws/internal/profile\"\n\t\"bws/internal/sandbox\"\n\n\t\"github.com/fatih/color\"\n)\n\n" + print_info_func)

