#!/usr/bin/env ruby
# frozen_string_literal: true

# check_docs_drift.rb — verify docs/commands.md against the real CLI surface.
#
# The command reference is hand-written and has silently drifted once already
# (the clihelp v0.3.53 upgrade changed flags, aliases and descriptions without
# anyone updating the prose). This script makes that drift a build failure.
#
# It reads the machine-readable command tree from the built binary's hidden
# `inventory` command and compares it with the commands and flags named in
# docs/commands.md:
#
#   * a documented `bws <command>` that does not exist            -> error
#   * a real command with a heading missing from the reference    -> warning
#   * a documented `--flag` that the command does not declare      -> error
#
# Usage:
#   tools/check_docs_drift.rb [--binary PATH] [--strict] [--help]
#
# --strict turns "command missing from docs" warnings into errors. The default
# is to fail only on claims that are *wrong*, not on incomplete coverage, so
# the check is useful before the reference is exhaustive.

require 'json'
require 'open3'
require 'set'

ROOT = File.expand_path('..', __dir__)
Dir.chdir(ROOT)

options = { binary: File.join(ROOT, 'bws'), strict: false }
ARGV.each_with_index do |arg, i|
  case arg
  when '--binary' then options[:binary] = ARGV[i + 1]
  when '--strict' then options[:strict] = true
  when '-h', '--help'
    puts <<~USAGE
      Usage: tools/check_docs_drift.rb [--binary PATH] [--strict]

      Compares docs/commands.md with the built binary's command tree.
        --binary PATH  Use PATH as the bws binary (default: bws)
        --strict       Fail when a real command is missing from the docs
    USAGE
    exit 0
  end
end

DOC = 'docs/commands.md'

# skip_check? reasons the check cannot run meaningfully, and says which.
def skip_check?(binary)
  unless File.exist?(binary)
    puts "SKIP: #{binary} not found; run 'go build -o bws .' first."
    return true
  end
  unless File.exist?(DOC)
    puts "SKIP: #{DOC} not found."
    return true
  end
  false
end

# inventory runs the binary and parses the JSON tree. Returns nil on any failure
# so the caller can skip rather than crash.
def inventory(binary)
  out, status = Open3.capture2(binary, 'inventory')
  return nil unless status.success?

  JSON.parse(out)
rescue JSON::ParserError, Errno::ENOENT
  nil
end

# flatten_index returns { entry_index, canonical } where entry_index maps every
# full command path and alias path to its command entry, and canonical maps the
# same keys to the command's canonical path (so "gw list" -> "git-workflow list").
# Aliases are expanded recursively, so a heading written as "gw list" resolves
# even though the canonical path is "git-workflow list".
def flatten_index(commands, prefix = [], prefix_canonical = [], entry = {}, canonical = {})
  commands.each do |cmd|
    path = prefix + [cmd['name']]
    canon = prefix_canonical + [cmd['name']]
    key = path.join(' ')
    entry[key] = cmd
    canonical[key] = canon.join(' ')

    aliases = [cmd['name']] + (cmd['aliases'] || [])
    aliases.each do |al|
      akey = (prefix + [al]).join(' ')
      entry[akey] ||= cmd
      canonical[akey] ||= canon.join(' ')
    end

    # Recurse once per spelling of this command's path so subcommands inherit
    # every alias of every ancestor.
    aliases.each do |al|
      child_prefix = prefix + [al]
      child_canonical = prefix_canonical + [cmd['name']]
      flatten_index(cmd['subcommands'] || [], child_prefix, child_canonical, entry, canonical)
    end
  end
  [entry, canonical]
end

# heading_words returns the leading command words of a `bws ...` heading,
# stopping at the first placeholder ("<name>") or bracketed group ("[flags]").
def heading_words(spec)
  spec = spec.sub(/\Abws\s+/, '')
  leading = spec.split(/[<\[]/, 2).first.to_s
  leading.scan(/[a-z][a-z0-9-]*/)
end

# documented_commands extracts the `### `bws ...`` headings, returning the
# command words (without a leading "bws"), the line number, and whether the
# heading ends in an angle-bracket placeholder ("<shell>") that stands for the
# command's direct subcommands.
def documented_commands(text)
  found = []
  text.each_line.with_index(1) do |line, num|
    next unless (m = line.match(/^#{'#'}{3,4}\s+`bws\s+([^`]+)`/))

    words = heading_words(m[1])
    next if words.empty?

    # A placeholder right after the command words means the heading documents
    # every child (e.g. "bws config completion <shell>").
    rest = m[1].sub(/\Abws\s+/, '')
    covers = rest.match?(Regexp.new(Regexp.escape(words.join(' ')) + '\s+<'))
    found << { words: words, line: num, covers_children: covers }
  end
  found
end

# documented_flags extracts `--flag` mentions from option-table rows under each
# command heading, so a flag is checked only against the command it documents.
# Prose (which cites bwrap flags like --bind) and the aliases column are ignored.
def documented_flags(text)
  blocks = Hash.new { |h, k| h[k] = [] }
  current = nil
  text.each_line.with_index(1) do |line, num|
    if (m = line.match(/^#{'#'}{3,4}\s+`bws\s+([^`]+)`/))
      current = heading_words(m[1]).join(' ')
      next
    end
    next unless current && line.start_with?('|')

    line.scan(/\| `(--[a-z][a-z0-9-]*)/) { |(f)| blocks[current] << [f, num] }
  end
  blocks
end

puts '=== Checking docs/commands.md against the CLI ==='
exit 0 if skip_check?(options[:binary])

data = inventory(options[:binary])
if data.nil?
  puts "SKIP: could not read inventory from #{options[:binary]} (does the inventory command exist?)."
  exit 0
end

index, canonical = flatten_index(data['commands'] || [])
documented = documented_commands(File.read(DOC))
flags_by_cmd = documented_flags(File.read(DOC))
globals = (data['global_flags'] || []).to_set

errors = []
warnings = []

# resolve_path returns the longest command path that is a prefix of words, or
# nil. Headings carry arguments ("bws run <cmd> <args>") beyond the command.
def resolve_path(index, words)
  words.length.downto(1) do |len|
    candidate = words[0, len].join(' ')
    return candidate if index.key?(candidate)
  end
  nil
end

# 1. Documented commands that do not resolve.
documented.each do |entry|
  next if resolve_path(index, entry[:words])

  errors << "#{DOC}:#{entry[:line]}: `bws #{entry[:words].join(' ')}` is not a command"
end

# 2. Real commands missing from the reference. A heading written with an alias
# ("bws gw list") documents the canonical command ("git-workflow list"). A parent
# that has subcommands needs no heading of its own once its children are
# documented, and library-owned commands (manpage) are documented elsewhere.
LIBRARY_COMMANDS = %w[manpage].to_set
documented_paths = documented.filter_map { |e| resolve_path(index, e[:words]) }
                              .map { |p| canonical[p] || p }.to_set
# Paths whose heading ends in a placeholder and therefore documents its children.
covering_paths = documented.filter_map do |e|
  next unless e[:covers_children]

  p = resolve_path(index, e[:words])
  canonical[p] || p if p
end.to_set
canonical.each do |name, canon|
  next unless name == canon # only canonical paths, not aliases
  next if documented_paths.include?(canon)

  top = canon.split.first
  next if LIBRARY_COMMANDS.include?(top)
  # A parent is covered if any documented path extends it, and a child is
  # covered when its parent's heading carries a subcommand placeholder.
  next if documented_paths.any? { |p| p.start_with?("#{canon} ") }
  parent = canon.split[0..-2].join(' ')
  next if !parent.empty? && covering_paths.include?(parent)

  warnings << "command `bws #{canon}` exists but has no heading in #{DOC}"
end

# 3. Documented flags that the command does not declare. Global flags are
# accepted on every command, so they are always valid.
flags_by_cmd.each do |cmd_key, flags|
  entry = index[cmd_key] || index[canonical[cmd_key]]
  next unless entry

  declared = (entry['flags'] || []).to_set | globals
  flags.each do |flag, line|
    next if declared.include?(flag)

    errors << "#{DOC}:#{line}: `bws #{cmd_key}` documents #{flag}, which it does not declare"
  end
end

warnings.uniq!
warnings.each { |w| puts "WARNING: #{w}" }
errors.each { |e| puts "ERROR: #{e}" }

if options[:strict] && warnings.any?
  puts "\nFAIL: #{warnings.size} command(s) undocumented (strict mode)."
  exit 1
end
if errors.any?
  puts "\nFAIL: #{errors.size} documentation claim(s) do not match the CLI."
  exit 1
end

puts "\n#{DOC} matches the CLI surface."
exit 0
