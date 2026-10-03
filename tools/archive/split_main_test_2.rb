#!/usr/bin/env ruby

lines = File.readlines('main_test.go')

main_test = []
conf_test = ["package main\n\nimport (\n\t\"os\"\n\t\"os/exec\"\n\t\"path/filepath\"\n\t\"strings\"\n\t\"testing\"\n)\n\n"]

in_conf = false
lines.each do |line|
  if line.start_with?("func TestConf") || line.start_with?("func TestPath")
    in_conf = true
  end
  
  if in_conf
    conf_test << line
    if line == "}\n"
      in_conf = false
    end
  else
    main_test << line
  end
end

File.write('main_test.go', main_test.join)
File.write('main_config_cmds_test.go', conf_test.join)
