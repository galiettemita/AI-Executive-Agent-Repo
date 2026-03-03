package feature_flags

const (
	FlagSkillsRollout     = "skills.rollout"
	FlagLLMProviderSwitch = "llm.provider_switch"
	FlagCanaryFeatures    = "canary.features"
)

func DefaultSystemFlags() []Flag {
	return []Flag{
		{
			Key:      FlagSkillsRollout,
			FlagType: "ruleset",
			Enabled:  true,
		},
		{
			Key:      FlagLLMProviderSwitch,
			FlagType: "ruleset",
			Enabled:  true,
		},
		{
			Key:      FlagCanaryFeatures,
			FlagType: "ruleset",
			Enabled:  false,
		},
	}
}

func (s *Service) BootstrapSystemFlags() {
	for _, flag := range DefaultSystemFlags() {
		if _, exists := s.GetFlag(flag.Key); exists {
			continue
		}
		s.UpsertFlag(flag)
	}
}
