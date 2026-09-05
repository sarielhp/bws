package config

// DefaultHistoryMasks lists shell, REPL, database, and utility history files
// that are blocked from sandbox environments by default.
var DefaultHistoryMasks = []string{
	// Shell command histories
	"~/.bash_history",
	"~/.zsh_history",
	"~/.zhistory",
	"~/.histfile",
	"~/.sh_history",
	"~/.ash_history",
	"~/.history",
	"~/.fish_history",
	"~/.local/share/fish/fish_history",
	"~/.config/fish/fish_history",
	"~/.local/state/bash/history",
	"~/.local/share/zsh/history",
	"~/.config/nushell/history.txt",
	"~/.local/share/nushell/history.txt",
	"~/.local/share/powershell/PSReadLine/ConsoleHost_history.txt",
	"~/.xonsh_history.json",
	"~/.local/share/blesh/history",

	// REPLs and language tools
	"~/.python_history",
	"~/.local/state/python/history",
	"~/.node_repl_history",
	"~/.irb_history",
	"~/.pry_history",
	"~/.julia_history",
	"~/.Rhistory",
	"~/.php_history",
	"~/.ghci_history",
	"~/.erlang_history",
	"~/.iex_history",
	"~/.lua_history",

	// Database and CLI tools
	"~/.psql_history",
	"~/.mysql_history",
	"~/.sqlite_history",
	"~/.rediscli_history",
	"~/.dbshell",

	// Utilities, pagers and editors
	"~/.gdb_history",
	"~/.lldb_history",
	"~/.lesshst",
	"~/.local/state/lesshst",
	"~/.nano_history",
	"~/.viminfo",
}

// HistoryMaskEnabled returns true if default command history masking is active.
// Defaults to true unless explicitly disabled in features.
func HistoryMaskEnabled(cfg *Config) bool {
	return FeatureEnabledDefault(cfg, func(f *FeaturesConfig) *bool { return f.MaskHistory }, true)
}
