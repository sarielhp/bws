#!/usr/bin/env ruby

content = File.read('internal/gitworkflow/gitworkflow.go')
parts = content.split(/^func getGitRootDir/)

header = parts[0]
helpers = "func getGitRootDir" + parts[1]

File.write('internal/gitworkflow/gitworkflow.go', header)
File.write('internal/gitworkflow/helpers.go', "package gitworkflow\n\nimport (\n\t\"bytes\"\n\t\"fmt\"\n\t\"os\"\n\t\"os/exec\"\n\t\"path/filepath\"\n\t\"strings\"\n)\n\n" + helpers)
