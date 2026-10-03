#!/usr/bin/env ruby

lines = File.readlines('main_test.go')

main_test = []
safety_test = ["package main\n\nimport (\n\t\"os\"\n\t\"os/exec\"\n\t\"path/filepath\"\n\t\"strings\"\n\t\"testing\"\n)\n\n"]

in_safety = false
lines.each do |line|
  if line.start_with?("func TestSafety")
    in_safety = true
  end
  
  if in_safety
    safety_test << line
    if line == "}\n"
      in_safety = false
    end
  else
    main_test << line
  end
end

File.write('main_test.go', main_test.join)
File.write('main_safety_test.go', safety_test.join)
