#!/usr/bin/env ruby
# frozen_string_literal: true

require 'open3'

ROOT = File.expand_path('..', __dir__)
Dir.chdir(ROOT)

FILE_SOFT_LIMIT = 800
FILE_HARD_LIMIT = 1100
FUNC_HARD_LIMIT = 80

files = Dir.glob('**/*.go').reject { |f| f.start_with?('.git/', 'vendor/') }

has_error = false
warnings = []
func_errors = []

puts '=== Auditing Go file & function line counts ==='

changed_files = []
begin
  branch, _ = Open3.capture2('git', 'rev-parse', '--abbrev-ref', 'HEAD')
  ref = branch.strip == 'main' ? 'HEAD~1..HEAD' : 'origin/main...HEAD'
  out, status = Open3.capture2('git', 'diff', '--name-only', ref)
  changed_files = out.lines.map(&:strip) if status.success?
rescue StandardError
  # ignore
end

files.each do |f|
  lines = File.readlines(f)
  line_count = lines.size

  if line_count > FILE_HARD_LIMIT
    puts "ERROR: #{f} has #{line_count} lines (hard limit: #{FILE_HARD_LIMIT})"
    has_error = true
  elsif line_count > FILE_SOFT_LIMIT
    warnings << "WARNING: #{f} has #{line_count} lines (soft limit: #{FILE_SOFT_LIMIT})"
  end

  in_func = false
  func_name = ''
  start_line = 0
  brace_count = 0

  lines.each_with_index do |line, idx|
    line_num = idx + 1
    if !in_func
      if line =~ /^func\s+(\([^)]+\)\s+)?([A-Za-z0-9_]+)/
        in_func = true
        func_name = line.strip
        start_line = line_num
        brace_count = line.count('{') - line.count('}')
        in_func = false if brace_count.zero? && line.include?('{')
      end
    else
      brace_count += line.count('{') - line.count('}')
      if brace_count <= 0
        func_len = line_num - start_line + 1
        if func_len > FUNC_HARD_LIMIT
          msg = "#{f}:#{start_line} (#{func_len} lines, limit #{FUNC_HARD_LIMIT}): #{func_name}"
          if changed_files.include?(f)
            puts "ERROR: Function in changed file exceeds limit: #{msg}"
            has_error = true
          else
            func_errors << msg
          end
        end
        in_func = false
      end
    end
  end
end

warnings.each { |w| puts w }
if func_errors.any?
  puts "\nExisting legacy functions exceeding limit:"
  func_errors.each { |e| puts "  #{e}" }
end

if has_error
  puts "\nFAIL: Line count audit failed."
  exit 1
end

puts "\nAll audited files and modified functions within limits."
exit 0
