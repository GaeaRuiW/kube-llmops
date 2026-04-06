package builder

// EnginePort returns the default port for a given engine.
func EnginePort(engine string) int32 {
	if cfg, ok := engines[engine]; ok {
		return cfg.Port
	}
	return 8000
}
