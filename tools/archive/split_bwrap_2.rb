#!/usr/bin/env ruby

lines = File.readlines('internal/bwrap/bwrap.go')
bwrap_go = []
binds_go = []

in_binds = false
i = 0
while i < lines.length
  line = lines[i]
  if line.include?("type bindItem struct")
    in_binds = true
    bwrap_go << "\targs = append(args, buildBinds(cfg, sandboxDir, homeDir, verbose)...)\n"
    binds_go << "package bwrap\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"sort\"\n\t\"strings\"\n\t\"bws/internal/config\"\n\t\"bws/internal/util\"\n)\n\nfunc buildBinds(cfg *config.Config, sandboxDir, homeDir string, verbose bool) []string {\n\tvar args []string\n"
    binds_go << line
  elsif in_binds
    binds_go << line
    if line.include?("args = append(args, flag, b.host, b.dest)")
      # we need to include the verbose print as well
      i += 1
      binds_go << lines[i]
      i += 1
      binds_go << lines[i]
      i += 1
      binds_go << lines[i]
      binds_go << "\treturn args\n}\n"
      in_binds = false
    end
  else
    bwrap_go << line
  end
  i += 1
end

File.write('internal/bwrap/bwrap.go', bwrap_go.join)
File.write('internal/bwrap/binds.go', binds_go.join)
