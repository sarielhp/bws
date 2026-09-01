#!/usr/bin/env ruby

lines = File.readlines('internal/bwrap/bwrap.go')

start_idx = lines.index { |l| l.include?("type bindItem struct") }
end_idx = lines.index { |l| l.include?("args = append(args, flag, b.host, b.dest)") }

if start_idx && end_idx
  end_idx += 4
  
  extracted = lines[start_idx..end_idx]
  
  bwrap_go = lines[0...start_idx] + ["\targs = append(args, buildBinds(cfg, sandboxDir, homeDir, verbose)...)\n"] + lines[(end_idx + 1)..-1]
  
  binds_go = ["package bwrap\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"sort\"\n\t\"strings\"\n\t\"bws/internal/config\"\n\t\"bws/internal/util\"\n)\n\nfunc buildBinds(cfg *config.Config, sandboxDir, homeDir string, verbose bool) []string {\n\tvar args []string\n"] + extracted + ["\treturn args\n}\n"]
  
  File.write('internal/bwrap/bwrap.go', bwrap_go.join)
  File.write('internal/bwrap/binds.go', binds_go.join)
end
