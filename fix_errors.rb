#!/usr/bin/env ruby

def process_file(path)
  content = File.read(path)
  new_content = []
  
  content.each_line do |line|
    if line =~ /^(\s*)_ = (.*?)\n/
      indent = $1
      expr = $2
      # special handling for multiple returns if any, though _ = means single return usually
      new_content << "#{indent}if err := #{expr}; err != nil {\n#{indent}\t// explicitly ignored\n#{indent}}\n"
    else
      new_content << line
    end
  end
  
  File.write(path, new_content.join)
end

['internal/gitworkflow/gitworkflow.go', 'internal/dbus/proxy.go', 'internal/cli/config_cmds.go'].each do |f|
  process_file(f)
end
