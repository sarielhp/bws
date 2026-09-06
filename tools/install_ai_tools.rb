#!/usr/bin/env ruby
# frozen_string_literal: true

require 'open3'
require 'fileutils'

def find_bin(name)
  ENV['PATH'].split(File::PATH_SEPARATOR).each do |dir|
    bin = File.join(dir, name)
    return bin if File.executable?(bin) && !File.directory?(bin)
  end
  nil
end

def run_cmd(cmd)
  stdout, stderr, status = Open3.capture3(cmd)
  [status.success?, stdout.strip, stderr.strip]
end

def python_installer
  return :uv if find_bin('uv')
  return :pipx if find_bin('pipx')
  :pip
end

def install_python_tool(bin_name, package_name, installer, extra_args = [])
  puts "Installing #{bin_name} (#{package_name}) via #{installer}..."
  cmd = case installer
        when :uv
          # Python 3.14 (Debian experimental) lacks wheels for packages like scipy.
          # Default to Python 3.12 if available on host.
          python_arg = find_bin('python3.12') ? '--python 3.12' : ''
          extra = extra_args.join(' ')
          "uv tool install #{python_arg} #{extra} #{package_name}".squeeze(' ')
        when :pipx
          python_arg = find_bin('python3.12') ? '--python python3.12' : ''
          "pipx install #{python_arg} #{package_name}".squeeze(' ')
        else
          "python3 -m pip install --user #{package_name}"
        end

  ok, out, err = run_cmd(cmd)
  if ok
    bin_path = find_bin(bin_name)
    puts "  [SUCCESS] Installed #{bin_name} to #{bin_path || '~/.local/bin'}"
    true
  else
    puts "  [FAILED] Installation of #{package_name} failed:"
    puts "    #{err.empty? ? out : err}"
    false
  end
end

puts '=== AI Tools Installer & Health Check ==='
puts

installer = python_installer
puts "Detected Python tool installer: #{installer}"
puts

# 1. Ollama
print 'Checking Ollama... '
ollama_path = find_bin('ollama')
if ollama_path
  _, version_out, = run_cmd("#{ollama_path} --version")
  puts "[INSTALLED] (#{ollama_path}) - #{version_out}"
else
  puts '[MISSING]'
  puts 'Installing Ollama via official installer (curl https://ollama.com/install.sh)...'
  system('curl -fsSL https://ollama.com/install.sh | sh')
end
puts

# 2. Aider
print 'Checking Aider (aider-chat)... '
aider_path = find_bin('aider')
if aider_path
  _, version_out, = run_cmd("#{aider_path} --version")
  puts "[INSTALLED] (#{aider_path}) - #{version_out}"
else
  puts '[MISSING]'
  install_python_tool('aider', 'aider-chat', installer)
end
puts

# 3. Simon Willison's LLM CLI
print 'Checking LLM CLI (llm)... '
llm_path = find_bin('llm')
if llm_path
  _, version_out, = run_cmd("#{llm_path} --version")
  puts "[INSTALLED] (#{llm_path}) - #{version_out}"
else
  puts '[MISSING]'
  install_python_tool('llm', 'llm', installer)
end
puts

# 4. Shell-GPT (sgpt)
print 'Checking Shell-GPT (sgpt)... '
sgpt_path = find_bin('sgpt')
if sgpt_path
  _, version_out, = run_cmd("env OPENAI_API_KEY=dummy #{sgpt_path} --version")
  puts "[INSTALLED] (#{sgpt_path}) - #{version_out}"
else
  puts '[MISSING]'
  install_python_tool('sgpt', 'shell-gpt', installer, ['--with', 'click'])
end
puts

# 5. GitHub Copilot CLI (copilot / gh copilot)
print 'Checking GitHub Copilot CLI (copilot / gh copilot)... '
copilot_path = find_bin('copilot')
if copilot_path
  _, version_out, = run_cmd("#{copilot_path} --version")
  first_line = version_out.lines.first&.strip || version_out
  puts "[INSTALLED] (#{copilot_path}) - #{first_line}"
else
  puts '[MISSING]'
  if find_bin('npm')
    puts 'Installing GitHub Copilot CLI via npm (@github/copilot to ~/.local)...'
    ok, out, err = run_cmd('npm install --prefix ~/.local -g @github/copilot')
    if ok
      puts "  [SUCCESS] Installed copilot to #{find_bin('copilot') || '~/.local/bin/copilot'}"
    else
      puts "  [FAILED] Failed to install @github/copilot: #{err.empty? ? out : err}"
    end
  else
    puts '  [SKIPPED] npm not found; install Node.js/npm to install @github/copilot'
  end
end
puts

# 6. Continue CLI (cn)
print 'Checking Continue CLI (cn)... '
cn_path = find_bin('cn')
if cn_path
  _, version_out, = run_cmd("#{cn_path} --version")
  puts "[INSTALLED] (#{cn_path}) - #{version_out}"
else
  puts '[MISSING]'
  if find_bin('npm')
    puts 'Installing Continue CLI via npm (@continuedev/cli to ~/.local)...'
    ok, out, err = run_cmd('npm install --prefix ~/.local -g @continuedev/cli')
    if ok
      puts "  [SUCCESS] Installed cn to #{find_bin('cn') || '~/.local/bin/cn'}"
    else
      puts "  [FAILED] Failed to install @continuedev/cli: #{err.empty? ? out : err}"
    end
  else
    puts '  [SKIPPED] npm not found; install Node.js/npm to install @continuedev/cli'
  end
end
puts

puts '=== Summary of Status ==='
[
  ['Ollama', find_bin('ollama')],
  ['Aider', find_bin('aider')],
  ['LLM CLI', find_bin('llm')],
  ['Shell-GPT', find_bin('sgpt')],
  ['GitHub Copilot', find_bin('copilot') || (find_bin('gh') ? 'gh copilot' : nil)],
  ['Continue CLI', find_bin('cn')]
].each do |name, status|
  indicator = status ? '✓' : '✗'
  path_info = status ? "(#{status})" : 'Not installed'
  puts "  #{indicator} #{name.ljust(16)} : #{path_info}"
end
puts
