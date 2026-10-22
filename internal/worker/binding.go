package worker

import (
	"encoding/json"
	"time"

	"github.com/orpheus-agents/orpheus-space/internal/config"
	"github.com/orpheus-agents/orpheus-space/internal/schedule"
	"github.com/orpheus-agents/orpheus-space/internal/store/db"
)

func selectBinding(cfg config.Config, row db.Schedule, services []string) (*config.ExecutionBinding, map[string]any, error) {
	var selected *config.ExecutionBinding
	for _, service := range services {
		binding, ok := cfg.Execution.Bindings[service]
		if !ok {
			continue
		}
		if selected != nil || row.OwnerEmail == nil || row.Profile != binding.Profile || row.Template != binding.Template {
			return nil, nil, schedule.Invalid("services")
		}
		selected = new(binding)
	}
	if selected == nil {
		return nil, nil, nil
	}
	owner, ok := selected.Owners[*row.OwnerEmail]
	if !ok {
		return nil, nil, schedule.Invalid("owner_email")
	}
	return selected, owner.ScopeContext, nil
}

// bindRequest adds the explicit, operator-configured source authority accepted
// by a compatible core. Hook credentials remain in run-only services.
func bindRequest(raw []byte, binding *config.ExecutionBinding, scope map[string]any, row db.Schedule, occ db.ScheduleOccurrence, now time.Time) ([]byte, error) {
	if binding == nil {
		return raw, nil
	}
	subject := map[string]string{"namespace": binding.AuthorityNamespace, "id": row.ID.String()}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	access, err := json.Marshal(map[string]any{
		"invocation_id":          occ.ID,
		"subject":                subject,
		"scope_context":          scope,
		"allowed_input_subjects": []map[string]string{subject},
		"context_observed_at":    now.UTC(),
	})
	if err != nil {
		return nil, err
	}
	body["access_context"] = access
	context, err := json.Marshal(map[string]any{
		"schedule_id": row.ID, "occurrence_id": occ.ID,
		"owner_email": row.OwnerEmail, "scheduled_at": occ.ScheduledAt,
	})
	if err != nil {
		return nil, err
	}
	env, err := json.Marshal(map[string]string{"ORPHEUS_SPACE_CONTEXT": string(context)})
	if err != nil {
		return nil, err
	}
	hookServices, err := json.Marshal(binding.HookServices)
	if err != nil {
		return nil, err
	}
	body["env"] = env
	body["services"] = hookServices
	return json.Marshal(body)
}
