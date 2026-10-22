package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecutionBindingConfiguration(t *testing.T) {
	instructions := filepath.Join(t.TempDir(), "instructions.md")
	if err := os.WriteFile(instructions, []byte("Prepare report"), 0600); err != nil {
		t.Fatal(err)
	}
	valid := func() map[string]ExecutionBinding {
		return map[string]ExecutionBinding{"report": {Profile: "reports", Template: "native", AuthorityNamespace: "scheduler:space", InstructionsFile: instructions, HookServices: []string{"delivery"}, Owners: map[string]ExecutionOwner{"alice@example.test": {ScopeContext: map[string]any{"actor": 42}}}}}
	}
	bindings := valid()
	if err := validateBindings(bindings); err != nil || bindings["report"].Instructions == nil || *bindings["report"].Instructions != "Prepare report" {
		t.Fatal(bindings, err)
	}
	for _, change := range []func(*ExecutionBinding){
		func(b *ExecutionBinding) { b.Profile = "" },
		func(b *ExecutionBinding) { b.Template = "" },
		func(b *ExecutionBinding) { b.AuthorityNamespace = "invalid namespace" },
		func(b *ExecutionBinding) { b.Owners = nil },
		func(b *ExecutionBinding) {
			b.Owners = map[string]ExecutionOwner{"bad": {ScopeContext: map[string]any{"actor": 1}}}
		},
		func(b *ExecutionBinding) {
			b.Owners = map[string]ExecutionOwner{"ALICE@example.test": {ScopeContext: map[string]any{"actor": 1}}}
		},
		func(b *ExecutionBinding) { b.Owners = map[string]ExecutionOwner{"alice@example.test": {}} },
		func(b *ExecutionBinding) { b.Instructions = new("both") },
		func(b *ExecutionBinding) { b.InstructionsFile = "missing" },
		func(b *ExecutionBinding) { b.HookServices = []string{"delivery", "delivery"} },
	} {
		bindings := valid()
		b := bindings["report"]
		change(&b)
		bindings["report"] = b
		if err := validateBindings(bindings); err == nil {
			t.Fatal("invalid binding accepted")
		}
	}
}
