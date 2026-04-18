package core

// SkillManifest declares the capabilities a skill requires to execute.
// It is the machine-checkable contract between the skill author, the
// platform operator, and the end user.
type SkillManifest struct {
	// Name uniquely identifies the skill.
	Name string `json:"name"`
	// RequiresBackends lists backend name patterns the skill needs (e.g., "cheap", "mid").
	RequiresBackends []string `json:"requires_backends"`
	// RequiresTools lists tool name patterns the skill needs.
	RequiresTools []string `json:"requires_tools"`
	// RequiresLabels lists data labels the skill may process (e.g., "pii").
	RequiresLabels []string `json:"requires_labels"`
}
