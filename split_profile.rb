#!/usr/bin/env ruby

content = File.read('internal/profile/profile.go')
parts = content.split(/^func DetectProfiles/)

header = parts[0]
detect_funcs = "func DetectProfiles" + parts[1]

File.write('internal/profile/profile.go', header)
File.write('internal/profile/detect.go', "package profile\n\nimport (\n\t\"os\"\n\t\"path/filepath\"\n\t\"strings\"\n)\n\n" + detect_funcs)
