#!/usr/bin/env ruby

lines = File.readlines('internal/bwrap/bwrap.go')

# Let's write everything into bwrap.go but move the block out
new_bwrap = []
binds_go = ["package bwrap\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"sort\"\n\t\"strings\"\n\t\"bws/internal/config\"\n\t\"bws/internal/util\"\n)\n\nfunc buildBinds(cfg *config.Config, sandboxDir, homeDir string, verbose bool) []string {\n\tvar args []string\n"]

in_binds = false
i = 0
while i < lines.length
  line = lines[i]
  if line =~ /type bindItem struct/
    in_binds = true
    new_bwrap << "\targs = append(args, buildBinds(cfg, sandboxDir, homeDir, verbose)...)\n"
    binds_go << line
  elsif line =~ /for _, b := range allBinds/
    binds_go << line
    i += 1
    while i < lines.length
      l2 = lines[i]
      binds_go << l2
      if l2 =~ /args = append\(args, flag, b.host, b.dest\)/
         # wait 2 lines to close loop
         i += 1; binds_go << lines[i]
         i += 1; binds_go << lines[i] if lines[i] =~ /}/
         break
      end
      i += 1
    end
    binds_go << "\treturn args\n}\n"
    in_binds = false
  elsif in_binds
    binds_go << line
  else
    new_bwrap << line
  end
  i += 1
end

File.write('internal/bwrap/bwrap.go', new_bwrap.join)
File.write('internal/bwrap/binds.go', binds_go.join)
