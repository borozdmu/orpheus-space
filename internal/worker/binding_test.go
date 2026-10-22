package worker

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/orpheus-agents/orpheus-space/internal/config"
	"github.com/orpheus-agents/orpheus-space/internal/store/db"
)

func bindingFixture() (config.Config, db.Schedule, db.ScheduleOccurrence) {
	cfg := config.Config{}
	cfg.Execution.Bindings = map[string]config.ExecutionBinding{"report-delivery": {
		Profile: "reports", Template: "native", Instructions: new("Report instructions"),
		BeforeRun: new("prepare"), AfterRun: new("deliver"), HookServices: []string{"delivery-secret"},
		AuthorityNamespace: "scheduler:space", Owners: map[string]config.ExecutionOwner{
			"alice@example.test": {ScopeContext: map[string]any{"actor": 42, "readers": []int{42, 43}}},
		},
	}}
	row := db.Schedule{ID: uuid.New(), Timezone: "UTC", OwnerEmail: new("alice@example.test"), Profile: "reports", Template: "native", Services: []string{"report-delivery", "data"}, SessionMode: "reuse"}
	occ := db.ScheduleOccurrence{ID: uuid.New(), ScheduledAt: time.Now()}
	return cfg, row, occ
}

func TestBoundSnapshotAuthorityAndHookIsolation(t *testing.T) {
	cfg, row, occ := bindingFixture()
	now := time.Now().UTC()
	first, err := Builder(cfg)(row, occ, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Access struct {
			InvocationID uuid.UUID         `json:"invocation_id"`
			Subject      map[string]string `json:"subject"`
			Scope        map[string]any    `json:"scope_context"`
			Observed     time.Time         `json:"context_observed_at"`
		} `json:"access_context"`
		Configuration struct {
			Agent struct {
				Instructions string `json:"instructions"`
			} `json:"agent"`
			Sandbox struct {
				Services []string `json:"services"`
			} `json:"sandbox"`
			Hooks map[string]any `json:"hooks"`
		} `json:"configuration"`
		Services []string          `json:"services"`
		Env      map[string]string `json:"env"`
	}
	if err := json.Unmarshal(first.Body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Access.InvocationID != occ.ID || payload.Access.Subject["id"] != row.ID.String() || payload.Access.Subject["namespace"] != "scheduler:space" || !payload.Access.Observed.Equal(now) || payload.Access.Scope["actor"] != float64(42) {
		t.Fatal(payload.Access)
	}
	if payload.Configuration.Agent.Instructions != "Report instructions" || payload.Configuration.Hooks["after_run"] != "deliver" || len(payload.Services) != 1 || payload.Services[0] != "delivery-secret" {
		t.Fatal(payload)
	}
	for _, service := range payload.Configuration.Sandbox.Services {
		if service == "delivery-secret" {
			t.Fatal("hook-only credential exposed to harness")
		}
	}
	if payload.Env["ORPHEUS_SPACE_CONTEXT"] == "" {
		t.Fatal("missing hook context")
	}
	row.ReusableSessionID, row.ReusableFingerprint = new(uuid.New()), new(first.Fingerprint)
	occ.ID = uuid.New()
	reused, err := Builder(cfg)(row, occ, nil, now.Add(time.Minute))
	if err != nil || reused.Fingerprint != first.Fingerprint || reused.Path == "/api/v1/sessions" {
		t.Fatal(reused, err)
	}
	if err := json.Unmarshal(reused.Body, &payload); err != nil || payload.Access.InvocationID != occ.ID {
		t.Fatal(payload, err)
	}
	binding := cfg.Execution.Bindings["report-delivery"]
	binding.Owners["alice@example.test"] = config.ExecutionOwner{ScopeContext: map[string]any{"actor": 99}}
	changed, err := Builder(cfg)(row, occ, nil, now)
	if err != nil || changed.Fingerprint == first.Fingerprint || changed.Path != "/api/v1/sessions" {
		t.Fatal(changed, err)
	}
}

func TestBoundSnapshotRejectsUnknownAuthorityAndAmbiguity(t *testing.T) {
	for _, change := range []func(*config.Config, *db.Schedule){
		func(_ *config.Config, row *db.Schedule) { row.OwnerEmail = nil },
		func(_ *config.Config, row *db.Schedule) { row.OwnerEmail = new("unknown@example.test") },
		func(_ *config.Config, row *db.Schedule) { row.Profile = "other" },
		func(_ *config.Config, row *db.Schedule) { row.Template = "other" },
		func(cfg *config.Config, row *db.Schedule) {
			cfg.Execution.Bindings["other-delivery"] = cfg.Execution.Bindings["report-delivery"]
			row.Services = append(row.Services, "other-delivery")
		},
	} {
		cfg, row, occ := bindingFixture()
		change(&cfg, &row)
		if _, err := Builder(cfg)(row, occ, nil, time.Now()); err == nil {
			t.Fatal("unsafe binding accepted")
		}
	}
}
