#!/usr/bin/env ruby
# frozen_string_literal: true
#
# migrate_clihelp.rb — one-shot codemod for the clihelp v0.3.5 -> v0.3.52 upgrade.
#
# Rewrites test call sites that moved out of the main `clihelp` package:
#
#   clihelp.TestExecute(app, args)          -> clihelptest.Execute(app, args)
#   clihelp.TestExecuteWithStdin(app, ...)  -> clihelptest.ExecuteWithStdin(app, ...)
#
# and ensures a `clihelptest` import is present in any file it touches. The
# `clihelp` import is left in place (these files still use clihelp.App etc.).
#
# Usage:
#   tools/migrate_clihelp.rb [--dry-run] [path ...]
#
# With no paths it walks every *_test.go under the working directory.

require 'optparse'

options = { dry_run: false }
OptionParser.new do |o|
  o.banner = 'Usage: migrate_clihelp.rb [--dry-run] [path ...]'
  o.on('--dry-run', 'Show what would change without writing') { options[:dry_run] = true }
  o.on('-h', '--help') { puts o; exit }
end.parse!

ROOTS = ARGV.empty? ? ['.'] : ARGV
IMPORT = "\t\"github.com/sarielhp/clihelp/clihelptest\"\n"

def test_files(roots)
  files = roots.flat_map do |root|
    if File.directory?(root)
      Dir.glob(File.join(root, '**', '*_test.go'))
    else
      [root]
    end
  end
  files.select { |f| File.exist?(f) }.uniq.sort
end

def add_import(src)
  return src if src.include?('sarielhp/clihelp/clihelptest')
  return src unless src.sub!(%r{^(\t"github\.com/sarielhp/clihelp"\n)}m, "\\1#{IMPORT}")
  src
end

changed = []
test_files(ROOTS).each do |file|
  original = File.read(file)
  mutated = original.gsub(/clihelp\.TestExecuteWithStdin\b/, 'clihelptest.ExecuteWithStdin')
                     .gsub(/clihelp\.TestExecute\b/, 'clihelptest.Execute')

  next if mutated == original
  mutated = add_import(mutated)

  changed << file
  if options[:dry_run]
    puts "would update: #{file}"
  else
    File.write(file, mutated)
    puts "updated: #{file}"
  end
end

puts changed.empty? ? 'no files needed changes' : "#{changed.size} file(s) #{options[:dry_run] ? 'would be' : ''} updated"
