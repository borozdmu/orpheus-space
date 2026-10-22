package config

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"

	"github.com/orpheus-agents/orpheus-space/internal/schedule"
)

// ExecutionBinding is an operator-owned capability selected by a catalog service.
// Prompts and email similarity never create identity or service authority.
type ExecutionBinding struct {
	Profile            string                    `toml:"profile"`
	Template           string                    `toml:"template"`
	Instructions       *string                   `toml:"instructions"`
	InstructionsFile   string                    `toml:"instructions_file"`
	BeforeRun          *string                   `toml:"before_run"`
	AfterRun           *string                   `toml:"after_run"`
	HookServices       []string                  `toml:"hook_services"`
	AuthorityNamespace string                    `toml:"authority_namespace"`
	Owners             map[string]ExecutionOwner `toml:"owners"`
}

type ExecutionOwner struct {
	ScopeContext map[string]any `toml:"scope_context"`
}

var bindingName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

func validateBindings(bindings map[string]ExecutionBinding) error {
	for code, binding := range bindings {
		if !bindingName.MatchString(code) || strings.TrimSpace(binding.Profile) == "" || strings.TrimSpace(binding.Template) == "" || len(binding.Owners) == 0 {
			return errors.New("invalid execution binding")
		}
		if binding.AuthorityNamespace == "" || len(binding.AuthorityNamespace) > 128 || strings.ContainsAny(binding.AuthorityNamespace, "\r\n\t ") {
			return errors.New("invalid execution authority namespace")
		}
		for email, owner := range binding.Owners {
			normalized, err := schedule.Email(email)
			raw, marshalErr := json.Marshal(owner.ScopeContext)
			if err != nil || email != normalized || len(owner.ScopeContext) == 0 || marshalErr != nil || len(raw) > 16384 {
				return errors.New("invalid execution owner mapping")
			}
		}
		seen := map[string]bool{}
		for _, service := range binding.HookServices {
			if !bindingName.MatchString(service) || seen[service] {
				return errors.New("invalid execution hook services")
			}
			seen[service] = true
		}
		if binding.InstructionsFile != "" {
			if binding.Instructions != nil {
				return errors.New("choose binding instructions or instructions_file")
			}
			raw, err := os.ReadFile(binding.InstructionsFile)
			if err != nil {
				return errors.New("cannot read binding instructions_file")
			}
			binding.Instructions = new(string(raw))
		}
		bindings[code] = binding
	}
	return nil
}
