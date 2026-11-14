# frozen_string_literal: true

# Copied from ~/prog/standards/go/templates/settings.rb and adapted for bws.
module ToolSettings
  # Name of the project's temporary-file tree, <tmpdir>/<user>/<name>/; nil uses the
  # repository directory's name.
  PROJECT_NAME = 'bws'
  TOOLS = {}.freeze # Optional exact executable paths, e.g. { 'go-audit' => '/path/go-audit' }
  BUILD_PACKAGES = ['./...'].freeze
  # The root main package builds bin/bws.
  ARTIFACTS = { 'bin/bws' => '.' }.freeze
  INSTALL_TARGETS = { 'bin/bws' => File.expand_path('~/bin/bws') }.freeze
  # main.go embeds VERSION, so a bump changes the binary.
  VERSION_FILE = 'VERSION'
  VERSION_KIND = :embedded
  VERSION_UPDATE = [].freeze
  UPDATE_TOOLCHAIN = true
  # docs/commands.md must match the CLI; the check reads a freshly built binary.
  FAST_CHECKS = [
    %w[tools/build],
    %w[tools/check_docs_drift.rb --strict --binary bin/bws]
  ].freeze
  FULL_CHECKS = [].freeze
  TEST_TIMEOUT = '180s'
  VERIFICATION_INPUTS = [].freeze # integrations/ is tracked, so already digested
end
