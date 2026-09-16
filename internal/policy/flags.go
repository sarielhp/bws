package policy

import "bws/internal/config"

// Flags are explicit launch or export overrides applied after profile resolution.
type Flags struct {
	NoSSH   bool
	NoNet   bool
	Proxy   bool
	NoProxy bool
	DBus    bool
	NoDBus  bool
}

// Apply updates only settings explicitly requested by flags.
func (f Flags) Apply(cfg *config.Config) {
	if cfg.Features == nil {
		cfg.Features = &config.FeaturesConfig{}
	}
	yes, no := true, false
	if f.NoSSH {
		cfg.Features.EnableSSH = &no
	}
	if f.NoNet {
		cfg.Features.NoNet = &yes
	}
	if f.NoProxy {
		cfg.Features.EnableProxy = &no
	} else if f.Proxy {
		cfg.Features.EnableProxy = &yes
	}
	if f.NoDBus {
		cfg.Features.EnableDBus = &no
	} else if f.DBus {
		cfg.Features.EnableDBus = &yes
	}
}
