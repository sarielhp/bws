#!/usr/bin/env ruby

def process_file(path)
  content = File.read(path)
  new_content = []
  
  content.each_line do |line|
    if line =~ /^(\s*)_ = (.*?)\n/
      indent = $1
      expr = $2
      new_content << "#{indent}if err := #{expr}; err != nil {\n#{indent}\t// explicitly ignored in test\n#{indent}}\n"
    else
      new_content << line
    end
  end
  
  File.write(path, new_content.join)
end

Dir.glob('internal/**/*_test.go').each do |f|
  process_file(f)
end
