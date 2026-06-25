package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHelpListsExpectedCommandsAndUnknownCommandFails(t *testing.T) {
	var out, err bytes.Buffer
	if code := Run([]string{"--help"}, &out, &err); code != 0 {
		t.Fatalf("help exit code = %d, stderr = %s", code, err.String())
	}
	for _, want := range []string{"candidate", "packet", "gates", "plan", "active", "rollback", "report", "apply", "evidence", "safety"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help output missing %q:\n%s", want, out.String())
		}
	}

	out.Reset()
	err.Reset()
	if code := Run([]string{"definitely-not-a-command"}, &out, &err); code == 0 {
		t.Fatalf("unknown command succeeded; stdout=%s stderr=%s", out.String(), err.String())
	}
}

func TestCandidateAndPacketValidation(t *testing.T) {
	f := newFixtureSet(t)

	assertRunOK(t, []string{"candidate", "validate", "--candidate", f.candidatePath})
	assertRunOK(t, []string{"packet", "validate", "--packet", f.packetPath})

	badCandidate := cloneMap(t, f.candidate)
	badCandidate["target_slot"] = "unknown_slot"
	badCandidatePath := f.writeJSON("candidate-invalid.json", badCandidate)
	assertRunFails(t, []string{"candidate", "validate", "--candidate", badCandidatePath}, "unknown target slot")

	missingGate := cloneMap(t, f.packet)
	missingGate["required_gate_roles"] = []string{"arena_promotion_gate"}
	missingGatePath := f.writeJSON("packet-missing-gate.json", missingGate)
	assertRunFails(t, []string{"packet", "validate", "--packet", missingGatePath}, "missing required gate")

	liveApply := cloneMap(t, f.packet)
	liveApply["dry_run_only"] = false
	liveApplyPath := f.writeJSON("packet-live-apply.json", liveApply)
	assertRunFails(t, []string{"packet", "validate", "--packet", liveApplyPath}, "dry_run_only")
}

func TestEvidenceDigestFreshnessAndCandidateChecks(t *testing.T) {
	f := newFixtureSet(t)

	digestMismatch := cloneMap(t, f.packet)
	refs := cloneSliceMap(t, digestMismatch["evidence"])
	refs[0]["sha256"] = strings.Repeat("0", 64)
	digestMismatch["evidence"] = refs
	assertRunFails(t, []string{"packet", "validate", "--packet", f.writeJSON("packet-digest-mismatch.json", digestMismatch)}, "digest mismatch")

	stale := cloneMap(t, f.packet)
	refs = cloneSliceMap(t, stale["evidence"])
	refs[0]["expires_at_utc"] = "2000-01-01T00:00:00Z"
	stale["evidence"] = refs
	assertRunFails(t, []string{"packet", "validate", "--packet", f.writeJSON("packet-stale.json", stale)}, "stale evidence")

	wrongCandidate := cloneMap(t, f.packet)
	refs = cloneSliceMap(t, wrongCandidate["evidence"])
	refs[0]["candidate_id"] = "different-candidate"
	wrongCandidate["evidence"] = refs
	assertRunFails(t, []string{"packet", "validate", "--packet", f.writeJSON("packet-candidate-mismatch.json", wrongCandidate)}, "candidate mismatch")
}

func TestGateEvaluationActivationRollbackReportApplyAndSafety(t *testing.T) {
	f := newFixtureSet(t)

	gatePath := filepath.Join(f.tmp, "promotion-gate.json")
	assertRunOK(t, []string{"gates", "evaluate", "--packet", f.packetPath, "--out", gatePath})
	gate := readMap(t, gatePath)
	if gate["status"] != "passed" || gate["promotion_allowed"] != true || gate["activation_plan_allowed"] != true {
		t.Fatalf("unexpected gate result: %#v", gate)
	}

	failedCrucible := cloneMap(t, f.packet)
	refs := cloneSliceMap(t, failedCrucible["evidence"])
	refs[1]["status"] = "failed"
	failedCrucible["evidence"] = refs
	failedGatePath := filepath.Join(f.tmp, "failed-gate.json")
	assertRunOK(t, []string{"gates", "evaluate", "--packet", f.writeJSON("packet-failed-crucible.json", failedCrucible), "--out", failedGatePath})
	failedGate := readMap(t, failedGatePath)
	if failedGate["status"] != "failed" || failedGate["promotion_allowed"] != false {
		t.Fatalf("failed crucible should block promotion: %#v", failedGate)
	}

	activationPath := filepath.Join(f.tmp, "activation-plan.json")
	assertRunOK(t, []string{"plan", "activate", "--packet", f.packetPath, "--out", activationPath})
	activation := readMap(t, activationPath)
	if activation["dry_run_only"] != true || activation["mutates_live_state"] != false {
		t.Fatalf("activation plan must be dry-run only: %#v", activation)
	}
	assertRunFails(t, []string{"plan", "activate", "--packet", f.packetPath, "--out", filepath.Join(f.root, "activation-plan.json")}, "under tmp")

	activeNextPath := filepath.Join(f.tmp, "active-stack.next.json")
	assertRunOK(t, []string{"active", "render", "--plan", activationPath, "--out", activeNextPath})
	activeNext := readMap(t, activeNextPath)
	slots := activeNext["slots"].(map[string]any)
	factory := slots["factory"].(map[string]any)
	if factory["component_id"] != "ao-foundry" {
		t.Fatalf("active render did not update factory slot: %#v", factory)
	}

	rollbackPath := filepath.Join(f.tmp, "rollback-plan.json")
	assertRunOK(t, []string{"rollback", "plan", "--active", f.activePath, "--candidate", f.candidatePath, "--out", rollbackPath})
	rollback := readMap(t, rollbackPath)
	if rollback["dry_run_only"] != true || len(rollback["verification_commands"].([]any)) == 0 {
		t.Fatalf("rollback plan missing dry-run verification: %#v", rollback)
	}

	reportPath := filepath.Join(f.tmp, "promotion-report.md")
	assertRunOK(t, []string{"report", "render", "--gate", gatePath, "--plan", activationPath, "--out", reportPath})
	reportBytes, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	report := string(reportBytes)
	if !strings.Contains(report, "AO Promoter Promotion Report") || !strings.Contains(report, "ao-foundry") {
		t.Fatalf("unexpected report:\n%s", report)
	}

	applyPath := filepath.Join(f.tmp, "apply-dry-run.json")
	assertRunOK(t, []string{"apply", "--plan", activationPath, "--dry-run", "--out", applyPath})
	apply := readMap(t, applyPath)
	if apply["status"] != "dry_run_complete" || apply["mutates_live_state"] != false || apply["active_stack_written"] != false {
		t.Fatalf("unexpected apply result: %#v", apply)
	}
	assertRunFails(t, []string{"apply", "--plan", activationPath, "--out", filepath.Join(f.tmp, "apply-live.json")}, "dry-run")

	safetyPath := filepath.Join(f.tmp, "readme-scan.json")
	assertRunOK(t, []string{"safety", "scan", "--path", f.safeDocPath, "--out", safetyPath})
	safety := readMap(t, safetyPath)
	if safety["status"] != "passed" {
		t.Fatalf("safe doc should pass: %#v", safety)
	}
	assertRunFails(t, []string{"safety", "scan", "--path", f.unsafeDocPath, "--out", filepath.Join(f.tmp, "unsafe-scan.json")}, "safety scan failed")
}

func TestEvidenceInspectReportsDigestStatus(t *testing.T) {
	f := newFixtureSet(t)
	var out, err bytes.Buffer
	if code := Run([]string{"evidence", "inspect", "--packet", f.packetPath}, &out, &err); code != 0 {
		t.Fatalf("inspect failed: code=%d stderr=%s", code, err.String())
	}
	if !strings.Contains(out.String(), "arena_promotion_gate") || !strings.Contains(out.String(), "digest=ok") {
		t.Fatalf("unexpected inspect output:\n%s", out.String())
	}
}

func TestCheckedInExamplesAreCovered(t *testing.T) {
	root := filepath.Join("..", "..")

	assertRunOK(t, []string{"candidate", "validate", "--candidate", filepath.Join(root, "examples/candidates/valid/ao-foundry-candidate.json")})
	assertRunOK(t, []string{"packet", "validate", "--packet", filepath.Join(root, "examples/packets/valid/ao-promoter-v0.1.json")})

	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unknown target slot",
			args:    []string{"candidate", "validate", "--candidate", filepath.Join(root, "examples/candidates/invalid/unknown-target-slot.json")},
			wantErr: "unknown target slot",
		},
		{
			name:    "missing crucible gate",
			args:    []string{"packet", "validate", "--packet", filepath.Join(root, "examples/packets/invalid/missing-crucible-gate.json")},
			wantErr: "missing required gate",
		},
		{
			name:    "stale arena gate",
			args:    []string{"packet", "validate", "--packet", filepath.Join(root, "examples/packets/invalid/stale-arena-gate.json")},
			wantErr: "stale evidence",
		},
		{
			name:    "digest mismatch",
			args:    []string{"packet", "validate", "--packet", filepath.Join(root, "examples/packets/invalid/digest-mismatch.json")},
			wantErr: "digest mismatch",
		},
		{
			name:    "candidate mismatch",
			args:    []string{"packet", "validate", "--packet", filepath.Join(root, "examples/packets/invalid/candidate-id-mismatch.json")},
			wantErr: "candidate mismatch",
		},
		{
			name:    "live apply default",
			args:    []string{"packet", "validate", "--packet", filepath.Join(root, "examples/packets/invalid/live-apply-default.json")},
			wantErr: "dry_run_only",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertRunFails(t, tc.args, tc.wantErr)
		})
	}
}

type fixtureSet struct {
	root          string
	tmp           string
	candidate     map[string]any
	packet        map[string]any
	candidatePath string
	packetPath    string
	activePath    string
	safeDocPath   string
	unsafeDocPath string
}

func newFixtureSet(t *testing.T) fixtureSet {
	t.Helper()
	root := t.TempDir()
	tmp := filepath.Join(root, "tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	f := fixtureSet{root: root, tmp: tmp}
	candidate := map[string]any{
		"schema_version":      "ao.promoter.candidate.v0.1",
		"candidate_id":        "ao-foundry",
		"display_name":        "AO Foundry",
		"component_kind":      "factory",
		"version":             "v0.1.0",
		"source_ref":          "github.com/uesugitorachiyo/ao-foundry@v0.1.0",
		"target_slot":         "factory",
		"target_stack_id":     "ao-stack-local",
		"trust_boundary":      "public-preview-local",
		"expected_gate_roles": requiredGateRoles(),
	}
	f.candidate = candidate
	f.candidatePath = f.writeJSON("candidate.json", candidate)
	active := map[string]any{
		"schema_version":     "ao.promoter.active-stack.v0.1",
		"stack_id":           "ao-stack-local",
		"created_at_utc":     "2026-06-25T00:00:00Z",
		"previous_stack_ref": "none",
		"promotion_history":  []any{},
		"trust_boundary":     "public-preview-local",
		"slots": map[string]any{
			"factory": map[string]any{
				"slot":            "factory",
				"component_id":    "ao-forge",
				"version":         "v0.1.0",
				"source_ref":      "github.com/uesugitorachiyo/ao-forge@v0.1.0",
				"activated_by":    "fixture",
				"activation_gate": "fixture",
				"rollback_ref":    "rollback://ao-forge",
			},
		},
	}
	f.activePath = f.writeJSON("active.json", active)
	evidence := makeEvidenceRefs(t, &f)
	packet := map[string]any{
		"schema_version":        "ao.promoter.packet.v0.1",
		"packet_id":             "ao-promoter-v0.1",
		"candidate":             candidate,
		"current_active_stack":  f.activePath,
		"required_gate_roles":   requiredGateRoles(),
		"evidence":              evidence,
		"freshness_policy":      map[string]any{"max_age_hours": 720},
		"promotion_policy":      map[string]any{"require_all_gates": true, "require_zero_safety_findings": true},
		"rollback_required":     true,
		"rollback_plan_present": true,
		"dry_run_only":          true,
	}
	f.packet = packet
	f.packetPath = f.writeJSON("packet.json", packet)
	f.safeDocPath = filepath.Join(root, "safe.md")
	if err := os.WriteFile(f.safeDocPath, []byte("# Safe\nNo credentials here.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.unsafeDocPath = filepath.Join(root, "unsafe.md")
	if err := os.WriteFile(f.unsafeDocPath, []byte("pass"+"word = \"fixture-value\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

func makeEvidenceRefs(t *testing.T, f *fixtureSet) []map[string]any {
	t.Helper()
	statuses := map[string]string{
		"arena_promotion_gate":     "passed",
		"crucible_hardening_gate":  "passed",
		"covenant_policy_decision": "allowed",
		"foundry_goal_readiness":   "ready",
		"forge_packet_summary":     "verified",
		"ao2_run_summary":          "passed",
		"public_safety_scan":       "passed",
		"rollback_plan_ready":      "ready",
	}
	var refs []map[string]any
	for _, role := range requiredGateRoles() {
		body := map[string]any{
			"schema_version":  "ao.promoter.evidence.v0.1",
			"role":            role,
			"status":          statuses[role],
			"candidate_id":    "ao-foundry",
			"target_stack_id": "ao-stack-local",
			"findings_count":  0,
		}
		path := f.writeJSON(role+".json", body)
		refs = append(refs, map[string]any{
			"role":           role,
			"path":           path,
			"schema_version": "ao.promoter.evidence-ref.v0.1",
			"sha256":         fileSHA256(t, path),
			"status":         statuses[role],
			"candidate_id":   "ao-foundry",
			"created_at_utc": "2026-06-25T00:00:00Z",
			"expires_at_utc": "2999-01-01T00:00:00Z",
			"authority":      "fixture",
		})
	}
	return refs
}

func (f fixtureSet) writeJSON(name string, value any) string {
	path := filepath.Join(f.root, name)
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, append(bytes, '\n'), 0o644); err != nil {
		panic(err)
	}
	return path
}

func assertRunOK(t *testing.T, args []string) {
	t.Helper()
	var out, err bytes.Buffer
	if code := Run(args, &out, &err); code != 0 {
		t.Fatalf("Run(%v) code=%d stdout=%s stderr=%s", args, code, out.String(), err.String())
	}
}

func assertRunFails(t *testing.T, args []string, wantErr string) {
	t.Helper()
	var out, err bytes.Buffer
	if code := Run(args, &out, &err); code == 0 {
		t.Fatalf("Run(%v) unexpectedly succeeded stdout=%s stderr=%s", args, out.String(), err.String())
	}
	if !strings.Contains(err.String(), wantErr) {
		t.Fatalf("Run(%v) stderr missing %q:\n%s", args, wantErr, err.String())
	}
}

func requiredGateRoles() []string {
	return []string{
		"arena_promotion_gate",
		"crucible_hardening_gate",
		"covenant_policy_decision",
		"foundry_goal_readiness",
		"forge_packet_summary",
		"ao2_run_summary",
		"public_safety_scan",
		"rollback_plan_ready",
	}
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func cloneMap(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	bytes, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(bytes, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func cloneSliceMap(t *testing.T, value any) []map[string]any {
	t.Helper()
	bytes, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	if err := json.Unmarshal(bytes, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func readMap(t *testing.T, path string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFutureDatesRemainFresh(t *testing.T) {
	expires, err := time.Parse(time.RFC3339, "2999-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if time.Now().After(expires) {
		t.Fatal("fixture freshness horizon expired")
	}
}
