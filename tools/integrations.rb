# frozen_string_literal: true

require 'fileutils'

# Agent integrations installed beside the bws binary: the Antigravity skill and
# the OpenCode slash commands. Each is copied only when that tool's
# configuration directory already exists.
module Integrations
  module_function

  def install(root = Dir.pwd)
    install_skill(root)
    install_opencode(root)
  end

  def install_skill(root)
    config = File.expand_path('~/.gemini/config')
    return unless File.directory?(config)

    target = File.join(config, 'skills', 'bws')
    FileUtils.rm_rf(target)
    FileUtils.mkdir_p(File.dirname(target))
    FileUtils.cp_r(File.join(root, 'integrations', 'skills', 'bws'), target)
    puts 'Installed Antigravity skill to ~/.gemini/config/skills/bws'
  end

  def install_opencode(root)
    config = File.expand_path('~/.config/opencode')
    return unless File.directory?(config)

    target = File.join(config, 'command')
    FileUtils.mkdir_p(target)
    FileUtils.cp_r(Dir.glob(File.join(root, 'integrations', 'opencode', 'command', '*')), target)
    puts 'Installed OpenCode slash commands to ~/.config/opencode/command/'
  end
end
