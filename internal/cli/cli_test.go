package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	for _, want := range []string{"candidate", "packet", "gates", "plan", "active", "rollback", "report", "apply", "evidence", "safety", "mission", "live-mutation", "docs-boundary"} {
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

	crlfEvidence := cloneMap(t, f.packet)
	refs = cloneSliceMap(t, crlfEvidence["evidence"])
	originalPath := refs[0]["path"].(string)
	originalBytes, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	crlfPath := filepath.Join(f.root, "arena-promotion-gate-crlf.json")
	if err := os.WriteFile(crlfPath, []byte(strings.ReplaceAll(string(originalBytes), "\n", "\r\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	refs[0]["path"] = crlfPath
	crlfEvidence["evidence"] = refs
	assertRunOK(t, []string{"packet", "validate", "--packet", f.writeJSON("packet-crlf-evidence.json", crlfEvidence)})

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
	assertRunFails(t, []string{"plan", "activate", "--packet", f.packetPath, "--out", filepath.Join("artifacts", "activation-plan.json")}, "under tmp")

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

func TestSafetyScanBuildsDetectorsOncePerScanForScaleFixture(t *testing.T) {
	f := newFixtureSet(t)
	scalePath := filepath.Join(f.root, "scanner-scale.md")
	lines := make([]string, 500)
	for i := range lines {
		lines[i] = "Safe promoter fixture line with public gate evidence and no authority expansion."
	}
	if err := os.WriteFile(scalePath, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(f.tmp, "scanner-scale.json")

	assertRunOK(t, []string{"safety", "scan", "--path", scalePath, "--out", outPath})
	packet := readMap(t, outPath)
	metrics, ok := packet["scanner_metrics"].(map[string]any)
	if !ok {
		t.Fatalf("safety scan should report scanner metrics for detector construction regression: %#v", packet)
	}
	if metrics["detector_construction_count"] != float64(1) {
		t.Fatalf("detectors should be constructed once per scan, got metrics: %#v", metrics)
	}
	if metrics["lines_scanned"] != float64(len(lines)) {
		t.Fatalf("scale fixture line count mismatch: %#v", metrics)
	}
	if packet["status"] != "passed" || packet["findings_count"].(float64) != 0 {
		t.Fatalf("scale fixture should remain safety-clean: %#v", packet)
	}
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

func TestReadmeDocumentsAOMissionGatewayNoPromotionBoundary(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(readme)
	for _, want := range []string{
		"AO Mission Gateway No-Promotion Readback",
		"gateway readbacks are no-promotion evidence",
		"Telegram and A2A intents cannot promote classes",
		"timeline compaction is readback only",
		"promotion_allowed=false",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("README missing AO Mission gateway no-promotion wording %q", want)
		}
	}
}

func TestAOMissionGatewayNoPromotionFixtureStaysReadOnly(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "examples", "evidence", "valid", "ao-mission-gateway-no-promotion.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture map[string]any
	if err := json.Unmarshal(body, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture["schema_version"] != "ao.promoter.mission-gateway-no-promotion.v0.1" ||
		fixture["status"] != "ready" ||
		fixture["gateway_authority"] != "intent_readback_only" {
		t.Fatalf("bad Mission gateway no-promotion fixture: %#v", fixture)
	}
	for _, key := range []string{
		"promotion_allowed",
		"activation_plan_allowed",
		"class_promotion_allowed",
		"safe_to_execute",
		"executes_work",
		"approves_work",
		"mutates_repositories",
		"provider_calls_allowed",
		"release_or_publish_allowed",
		"credential_use_allowed",
		"direct_main_mutation_allowed",
		"concurrent_mutation_allowed",
	} {
		if fixture[key] != false {
			t.Fatalf("Mission gateway no-promotion fixture %s = %#v, want false", key, fixture[key])
		}
	}
}

func TestAOMissionRollupSummaryBindsOperatorNoPromotionReadback(t *testing.T) {
	outPath := filepath.Join("tmp", "ao-mission-promotion-rollup-summary-test.json")
	t.Cleanup(func() { _ = os.Remove(outPath) })
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"mission", "rollup-summary",
		"--no-promotion", filepath.Join("..", "..", "examples", "evidence", "valid", "ao-mission-gateway-no-promotion.json"),
		"--out", outPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mission rollup summary failed: code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "mission_rollup_summary="+outPath) {
		t.Fatalf("stdout missing rollup output path: %s", stdout.String())
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary map[string]any
	if err := json.Unmarshal(body, &summary); err != nil {
		t.Fatal(err)
	}
	if summary["schema_version"] != "ao.promoter.mission-rollup-summary.v0.1" || summary["status"] != "ready" {
		t.Fatalf("bad mission rollup summary: %#v", summary)
	}
	if summary["promotion_count"] != float64(0) || summary["no_promotion_count"] != float64(1) || summary["promotion_allowed"] != false {
		t.Fatalf("mission rollup summary did not preserve no-promotion outcome: %#v", summary)
	}
	if summary["mission_no_promotion_rollup_bound"] != true || summary["promotion_rollup_status"] != "no_promotion" {
		t.Fatalf("mission rollup summary did not bind no-promotion rollup: %#v", summary)
	}
	if summary["operator_status"] != "no_promotion_requested" || summary["read_only_operator_status"] != true {
		t.Fatalf("mission rollup summary missing operator status: %#v", summary)
	}
	if !strings.Contains(fmt.Sprint(summary["operator_summary"]), "No promotion requested") ||
		!strings.Contains(fmt.Sprint(summary["operator_next_action"]), "keep AO Mission gateway") {
		t.Fatalf("mission rollup summary missing operator-facing wording: %#v", summary)
	}
	for _, key := range []string{
		"safe_to_execute",
		"executes_work",
		"approves_work",
		"mutates_repositories",
		"provider_calls_allowed",
		"release_or_publish_allowed",
		"credential_use_allowed",
		"direct_main_mutation_allowed",
		"concurrent_mutation_allowed",
	} {
		if summary[key] != false {
			t.Fatalf("mission rollup summary %s = %#v, want false", key, summary[key])
		}
	}
}

func TestConsumesWaveEAssuranceCompatibilityVectors(t *testing.T) {
	root := filepath.Join("..", "..")
	cases := []struct {
		name              string
		path              string
		schema            string
		edge              string
		producerKey       string
		expectedKey       string
		expectedSchema    string
		expectedStatusKey string
	}{
		{
			name:              "arena",
			path:              filepath.Join(root, "examples", "compatibility", "arena-benchmark-result-to-promoter-assurance-input-v0.1.json"),
			schema:            "ao.compatibility.arena-benchmark-result-to-promoter-assurance-input-vector.v1",
			edge:              "ao-arena.benchmark_result -> ao-promoter.assurance_input",
			producerKey:       "arena_benchmark_result",
			expectedKey:       "expected_promoter_assurance_input",
			expectedSchema:    "ao.promoter.assurance-input.v1",
			expectedStatusKey: "assurance_status",
		},
		{
			name:              "crucible",
			path:              filepath.Join(root, "examples", "compatibility", "crucible-failure-injection-to-promoter-assurance-input-v0.1.json"),
			schema:            "ao.compatibility.crucible-failure-injection-to-promoter-assurance-input-vector.v1",
			edge:              "ao-crucible.failure_injection_result -> ao-promoter.assurance_input",
			producerKey:       "crucible_failure_injection_result",
			expectedKey:       "expected_promoter_assurance_input",
			expectedSchema:    "ao.promoter.assurance-input.v1",
			expectedStatusKey: "assurance_status",
		},
		{
			name:              "sentinel",
			path:              filepath.Join(root, "examples", "compatibility", "sentinel-verdict-to-promoter-input-v0.1.json"),
			schema:            "ao.compatibility.sentinel-verdict-to-promoter-input-vector.v1",
			edge:              "ao-sentinel.sentinel_verdict -> ao-promoter.promotion_input",
			producerKey:       "sentinel_verdict",
			expectedKey:       "expected_promoter_promotion_input",
			expectedSchema:    "ao.promoter.promotion-input.v1",
			expectedStatusKey: "promotion_input_status",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vector := readMap(t, tc.path)
			if vector["schema_version"] != tc.schema || vector["edge"] != tc.edge {
				t.Fatalf("unexpected vector identity: %#v", vector)
			}
			producer, ok := vector[tc.producerKey].(map[string]any)
			if !ok || producer["status"] == "" {
				t.Fatalf("vector missing producer payload: %#v", vector)
			}
			expected, ok := vector[tc.expectedKey].(map[string]any)
			if !ok ||
				expected["schema_version"] != tc.expectedSchema ||
				expected[tc.expectedStatusKey] != "accepted" {
				t.Fatalf("vector missing Promoter expectation: %#v", vector)
			}
			boundaries := vector["authority_boundaries"].(map[string]any)
			for _, key := range []string{"promotion_requested", "promotion_granted", "safe_to_execute", "executes_work", "mutates_repositories", "calls_providers", "releases_or_deploys"} {
				if boundaries[key] != false {
					t.Fatalf("%s boundary %s = %#v, want false", tc.name, key, boundaries[key])
				}
			}
		})
	}
}

func TestProducesPromoterVerdictToCommandStatusVector(t *testing.T) {
	root := filepath.Join("..", "..")
	vector := readMap(t, filepath.Join(root, "examples", "compatibility", "promoter-verdict-to-command-status-v0.1.json"))
	if vector["schema_version"] != "ao.compatibility.promoter-verdict-to-command-status-vector.v1" ||
		vector["edge"] != "ao-promoter.promotion_verdict -> ao-command.promotion_status" {
		t.Fatalf("unexpected Promoter Command vector identity: %#v", vector)
	}
	verdict := vector["promoter_verdict"].(map[string]any)
	if verdict["schema_version"] != "ao.promoter.promotion-verdict.v1" ||
		verdict["status"] != "observed" ||
		verdict["promotion_requested"] != false ||
		verdict["promotion_granted"] != false {
		t.Fatalf("unexpected Promoter verdict: %#v", verdict)
	}
	expected := vector["expected_command_promotion_status"].(map[string]any)
	if expected["schema_version"] != "ao-command.promotion-status.v1" ||
		expected["status"] != "observed" ||
		expected["promotion_requested"] != false ||
		expected["promotion_granted"] != false {
		t.Fatalf("unexpected Command expectation: %#v", expected)
	}
	boundaries := vector["authority_boundaries"].(map[string]any)
	for _, key := range []string{"safe_to_execute", "executes_work", "approves_work", "mutates_repositories", "calls_providers", "releases_or_deploys"} {
		if boundaries[key] != false {
			t.Fatalf("Promoter Command vector boundary %s = %#v, want false", key, boundaries[key])
		}
	}
}

func TestMonth4ControlledLoopNoRSIVerdictFixtureDeniesPromotion(t *testing.T) {
	root := filepath.Join("..", "..")
	verdict := readMap(t, filepath.Join(root, "examples", "evidence", "valid", "month4-controlled-loop-no-rsi-verdict.json"))
	if verdict["schema_version"] != "ao.promoter.month4-controlled-loop-no-rsi-verdict.v0.1" ||
		verdict["status"] != "readiness_denied" ||
		verdict["promotion_requested"] != false ||
		verdict["promotion_granted"] != false ||
		verdict["rsi_authorized"] != false ||
		verdict["rsi_status"] != "denied" ||
		verdict["dry_run_only"] != true {
		t.Fatalf("unexpected Month 4 no-RSI verdict: %#v", verdict)
	}
	for _, key := range []string{
		"live_self_modification_allowed",
		"provider_execution_allowed",
		"external_beta_launched",
		"release_or_publish_allowed",
		"tag_or_upload_allowed",
		"deploy_allowed",
		"mutates_live_state",
	} {
		if verdict[key] != false {
			t.Fatalf("Month 4 no-RSI verdict %s = %#v, want false", key, verdict[key])
		}
	}
	evidence, ok := verdict["evidence"].(map[string]any)
	if !ok ||
		evidence["policy_gate"] != "human_approval_required" ||
		evidence["ao2_dry_run"] != "fixture_only_passed" ||
		evidence["rollback"] != "verified" ||
		evidence["control_plane_observation"] != "observed" ||
		evidence["command_readback"] != "read_only" ||
		evidence["sentinel_wording"] != "no_overclaim_checks_passed" {
		t.Fatalf("Month 4 no-RSI verdict missing evidence readback: %#v", verdict)
	}
	blockers, ok := verdict["readiness_blockers"].([]any)
	if !ok || len(blockers) == 0 {
		t.Fatalf("Month 4 no-RSI verdict must keep readiness denied with blockers: %#v", verdict)
	}
	if !strings.Contains(fmt.Sprint(verdict["operator_explanation"]), "RSI remains denied") ||
		!strings.Contains(fmt.Sprint(verdict["operator_next_action"]), "Do not request live self-modification") {
		t.Fatalf("Month 4 no-RSI verdict missing operator-safe wording: %#v", verdict)
	}
}

func TestMonth5OperatorWorkflowNoPromotionFixtureDeniesActivation(t *testing.T) {
	root := filepath.Join("..", "..")
	verdict := readMap(t, filepath.Join(root, "examples", "evidence", "valid", "month5-operator-workflow-no-promotion.json"))
	if verdict["schema_version"] != "ao.promoter.month5-operator-workflow-no-promotion.v0.1" ||
		verdict["status"] != "readiness_denied" ||
		verdict["subject"] != "month5-operator-workflow-hardening" ||
		verdict["promotion_requested"] != false ||
		verdict["promotion_granted"] != false ||
		verdict["rsi_authorized"] != false ||
		verdict["rsi_status"] != "denied" ||
		verdict["operator_workflow_only"] != true {
		t.Fatalf("unexpected Month 5 no-promotion verdict: %#v", verdict)
	}
	for _, key := range []string{
		"live_self_modification_allowed",
		"provider_execution_allowed",
		"external_beta_launched",
		"release_or_publish_allowed",
		"tag_or_upload_allowed",
		"deploy_allowed",
		"mutates_live_state",
	} {
		if verdict[key] != false {
			t.Fatalf("Month 5 no-promotion verdict %s = %#v, want false", key, verdict[key])
		}
	}
	evidence, ok := verdict["evidence"].(map[string]any)
	if !ok ||
		evidence["operator_workflow_source"] != "defined" ||
		evidence["command_readback"] != "visible" ||
		evidence["safe_next_work"] != "selected_without_release_authority" ||
		evidence["run_state"] != "read_only" ||
		evidence["policy_gate"] != "approval_required" ||
		evidence["sentinel_wording"] != "no_overclaim_checks_passed" {
		t.Fatalf("Month 5 no-promotion verdict missing evidence readback: %#v", verdict)
	}
	command, ok := verdict["expected_command_readback"].(map[string]any)
	if !ok ||
		command["schema_version"] != "ao-command.promotion-status.v1" ||
		command["promotion_requested"] != false ||
		command["promotion_granted"] != false ||
		command["rsi_authorized"] != false {
		t.Fatalf("Month 5 no-promotion verdict missing Command readback: %#v", verdict)
	}
	if !strings.Contains(fmt.Sprint(verdict["operator_explanation"]), "RSI remains denied") ||
		!strings.Contains(fmt.Sprint(verdict["operator_next_action"]), "next stable release train planning") {
		t.Fatalf("Month 5 no-promotion verdict missing operator-safe wording: %#v", verdict)
	}
}

func TestMonth6ReleaseReadinessNoPromotionFixtureDeniesReleaseAndActivation(t *testing.T) {
	root := filepath.Join("..", "..")
	verdict := readMap(t, filepath.Join(root, "examples", "evidence", "valid", "month6-release-readiness-no-promotion.json"))
	if verdict["schema_version"] != "ao.promoter.month6-release-readiness-no-promotion.v0.1" ||
		verdict["status"] != "no_release_readiness_recorded" ||
		verdict["subject"] != "month6-release-train-readiness" ||
		verdict["release_decision"] != "no_release" ||
		verdict["promotion_requested"] != false ||
		verdict["promotion_granted"] != false ||
		verdict["rsi_authorized"] != false ||
		verdict["rsi_status"] != "denied" {
		t.Fatalf("unexpected Month 6 no-release verdict: %#v", verdict)
	}
	pair, ok := verdict["current_public_release_pair"].(map[string]any)
	if !ok ||
		pair["ao2_version"] != "v0.5.1" ||
		pair["control_plane_version"] != "v0.1.15" {
		t.Fatalf("Month 6 no-release verdict missing current public pair: %#v", verdict)
	}
	for _, key := range []string{
		"live_self_modification_allowed",
		"provider_execution_allowed",
		"external_beta_launched",
		"release_or_publish_allowed",
		"tag_or_upload_allowed",
		"deploy_allowed",
		"new_binary_publication_allowed",
		"mutates_live_state",
		"mutates_repositories",
		"calls_providers",
		"inspects_credentials",
	} {
		if verdict[key] != false {
			t.Fatalf("Month 6 no-release verdict %s = %#v, want false", key, verdict[key])
		}
	}
	evidence, ok := verdict["evidence"].(map[string]any)
	if !ok ||
		evidence["release_readiness_inventory"] != "no_runtime_source_change" ||
		evidence["command_readback"] != "release_decision_no_release_visible" ||
		evidence["compatibility_matrix"] != "16_edges_16_tested_gate_false" {
		t.Fatalf("Month 6 no-release verdict missing evidence readback: %#v", verdict)
	}
	blockers, ok := verdict["readiness_blockers"].([]any)
	if !ok || len(blockers) < 3 {
		t.Fatalf("Month 6 no-release verdict must keep readiness denied with blockers: %#v", verdict)
	}
	command, ok := verdict["expected_command_readback"].(map[string]any)
	if !ok ||
		command["schema_version"] != "ao-command.promotion-status.v1" ||
		command["release_decision"] != "no_release" ||
		command["promotion_requested"] != false ||
		command["promotion_granted"] != false ||
		command["rsi_authorized"] != false {
		t.Fatalf("Month 6 no-release verdict missing Command readback: %#v", verdict)
	}
	explanation, _ := verdict["operator_explanation"].(string)
	if !strings.Contains(explanation, "Promotion is not requested") || !strings.Contains(explanation, "RSI remains denied") {
		t.Fatalf("Month 6 no-release verdict missing operator-safe wording: %#v", verdict)
	}
}

func TestAdoptionMonth1GateReadinessNoPromotionFixture(t *testing.T) {
	root := filepath.Join("..", "..")
	verdict := readMap(t, filepath.Join(root, "examples", "evidence", "valid", "adoption-month1-gate-readiness-no-promotion.json"))
	if verdict["schema_version"] != "ao.promoter.adoption-month1-gate-readiness-no-promotion.v0.1" ||
		verdict["status"] != "gate_ready_no_promotion" ||
		verdict["subject"] != "adoption-month1-evidence-freshness" ||
		verdict["compatibility_gate_state"] != "ready" ||
		verdict["compatibility_gate_active"] != false ||
		verdict["promotion_requested"] != false ||
		verdict["promotion_granted"] != false ||
		verdict["rsi_authorized"] != false ||
		verdict["external_beta_launched"] != false {
		t.Fatalf("unexpected adoption Month 1 gate-readiness verdict: %#v", verdict)
	}
	currentPair, ok := verdict["current_public_release_pair"].(map[string]any)
	if !ok ||
		currentPair["ao2_version"] != "v0.5.1" ||
		currentPair["ao2_tag_target"] != "80ec5321f42d4bab17d5e64fdae6aa099ba59d4a" ||
		currentPair["control_plane_version"] != "v0.1.15" ||
		currentPair["control_plane_tag_target"] != "f1702b387607566cac457458af9adb5871a5c412" {
		t.Fatalf("adoption Month 1 verdict missing current public pair: %#v", verdict)
	}
	evidence, ok := verdict["evidence"].(map[string]any)
	if !ok ||
		evidence["freshness_status"] != "fresh" ||
		evidence["compatibility_matrix"] != "16_edges_16_tested" ||
		evidence["gate_activation"] != "not_authorized" {
		t.Fatalf("adoption Month 1 verdict missing evidence readback: %#v", verdict)
	}
	blockers, ok := verdict["readiness_blockers"].([]any)
	if !ok || len(blockers) < 3 {
		t.Fatalf("adoption Month 1 verdict must retain readiness blockers: %#v", verdict)
	}
	command, ok := verdict["expected_command_readback"].(map[string]any)
	if !ok ||
		command["compatibility_gate_state"] != "ready" ||
		command["compatibility_gate_active"] != false ||
		command["promotion_granted"] != false ||
		command["rsi_authorized"] != false {
		t.Fatalf("adoption Month 1 verdict missing Command readback: %#v", verdict)
	}
	if !strings.Contains(fmt.Sprint(verdict["operator_explanation"]), "does not activate") ||
		!strings.Contains(fmt.Sprint(verdict["operator_next_action"]), "operator adoption drills") {
		t.Fatalf("adoption Month 1 verdict missing operator-safe wording: %#v", verdict)
	}
	for _, key := range []string{
		"live_self_modification_allowed",
		"provider_execution_allowed",
		"release_or_publish_allowed",
		"tag_or_upload_allowed",
		"deploy_allowed",
		"new_binary_publication_allowed",
		"mutates_live_state",
		"calls_providers",
		"inspects_credentials",
	} {
		if verdict[key] != false {
			t.Fatalf("adoption Month 1 verdict %s = %#v, want false", key, verdict[key])
		}
	}
}

func TestGitHubIssueWorkflowNoPromotionFixture(t *testing.T) {
	root := filepath.Join("..", "..")
	verdict := readMap(t, filepath.Join(root, "examples", "evidence", "valid", "github-issue-workflow-no-promotion.json"))
	if verdict["schema_version"] != "ao.promoter.github-issue-workflow-no-promotion.v0.1" ||
		verdict["status"] != "ready" ||
		verdict["subject"] != "github-issue-to-draft-pr-month1" ||
		verdict["promotion_requested"] != false ||
		verdict["promotion_granted"] != false ||
		verdict["external_beta_launched"] != false ||
		verdict["release_selected"] != false ||
		verdict["rsi_authorized"] != false ||
		verdict["readiness_does_not_imply_promotion"] != true {
		t.Fatalf("unexpected GitHub issue workflow no-promotion verdict: %#v", verdict)
	}
	pair := verdict["current_public_pair"].(map[string]any)
	if pair["ao2"] != "v0.5.1" || pair["control_plane"] != "v0.1.16" {
		t.Fatalf("GitHub issue workflow verdict has stale current pair: %#v", pair)
	}
	pr := verdict["feature_generated_pr_state"].(map[string]any)
	if pr["draft_pr_allowed_after_digest_approval"] != true ||
		pr["ready_for_review_allowed"] != false ||
		pr["merge_allowed"] != false ||
		pr["review_approval_allowed"] != false {
		t.Fatalf("GitHub issue workflow PR boundary widened: %#v", pr)
	}
	if verdict["github_issue_writes_allowed"] != false {
		t.Fatalf("GitHub issue writes must remain denied: %#v", verdict)
	}
	refs := verdict["evidence_refs"].([]any)
	if len(refs) != 3 {
		t.Fatalf("GitHub issue workflow verdict missing evidence refs: %#v", verdict)
	}
	command := verdict["operator_readback"].(map[string]any)
	if command["promotion_requested"] != false ||
		command["promotion_granted"] != false ||
		command["external_beta_launched"] != false ||
		command["rsi_authorized"] != false {
		t.Fatalf("GitHub issue workflow command readback widened authority: %#v", command)
	}
}

func TestAdoptionMonth2OperatorDrillNoPromotionFixture(t *testing.T) {
	root := filepath.Join("..", "..")
	verdict := readMap(t, filepath.Join(root, "examples", "evidence", "valid", "adoption-month2-operator-drill-no-promotion.json"))
	if verdict["schema_version"] != "ao.promoter.adoption-month2-operator-drill-no-promotion.v0.1" ||
		verdict["status"] != "ready" ||
		verdict["subject"] != "adoption-month2-operator-drills" ||
		verdict["compatibility_gate_state"] != "ready" ||
		verdict["compatibility_gate_active"] != false ||
		verdict["promotion_requested"] != false ||
		verdict["promotion_granted"] != false ||
		verdict["rsi_authorized"] != false ||
		verdict["external_beta_launched"] != false {
		t.Fatalf("unexpected adoption Month 2 operator drill verdict: %#v", verdict)
	}
	for _, key := range []string{
		"provider_pilot_ran",
		"release_or_publish",
		"tag_created",
		"upload_performed",
		"deployment_performed",
		"live_self_modification",
	} {
		if verdict[key] != false {
			t.Fatalf("adoption Month 2 verdict %s = %#v, want false", key, verdict[key])
		}
	}
	command := verdict["command_readback"].(map[string]any)
	if command["schema"] != "ao.command.operator-workflow-readback.v0.1" ||
		command["compatibility_gate_state"] != "ready" ||
		command["compatibility_gate_activation_authorized"] != false ||
		command["promotion_requested"] != false ||
		command["rsi_authorized"] != false {
		t.Fatalf("adoption Month 2 verdict missing Command readback: %#v", verdict)
	}
	if !strings.Contains(verdict["operator_summary"].(string), "does not imply promotion") ||
		!strings.Contains(verdict["operator_summary"].(string), "RSI remains denied") {
		t.Fatalf("adoption Month 2 verdict missing operator-safe wording: %#v", verdict)
	}
}

func TestAdoptionMonth3EvidenceMaintenanceNoPromotionFixture(t *testing.T) {
	root := filepath.Join("..", "..")
	verdict := readMap(t, filepath.Join(root, "examples", "evidence", "valid", "adoption-month3-evidence-maintenance-no-promotion.json"))
	if verdict["schema_version"] != "ao.promoter.adoption-month3-evidence-maintenance-no-promotion.v0.1" ||
		verdict["status"] != "ready" ||
		verdict["subject"] != "adoption-month3-evidence-maintenance" ||
		verdict["evidence_freshness_status"] != "fresh" ||
		verdict["compatibility_gate_state"] != "ready" ||
		verdict["compatibility_gate_active"] != false ||
		verdict["promotion_requested"] != false ||
		verdict["promotion_granted"] != false ||
		verdict["rsi_authorized"] != false ||
		verdict["external_beta_launched"] != false ||
		verdict["evidence_freshness_does_not_imply_promotion"] != true {
		t.Fatalf("unexpected adoption Month 3 maintenance verdict: %#v", verdict)
	}
	for _, key := range []string{
		"provider_pilot_ran",
		"release_or_publish",
		"tag_created",
		"upload_performed",
		"deployment_performed",
		"live_self_modification",
		"compatibility_gate_activation_authorized",
	} {
		if verdict[key] != false {
			t.Fatalf("adoption Month 3 verdict %s = %#v, want false", key, verdict[key])
		}
	}
	maintenance := verdict["maintenance_readback"].(map[string]any)
	if maintenance["current_release_metadata"] != "fresh" ||
		maintenance["matrix_drift"] != "none" ||
		maintenance["canonical_vectors"] != "16_present" ||
		maintenance["consumer_tests"] != "16_present" {
		t.Fatalf("adoption Month 3 verdict missing maintenance readback: %#v", verdict)
	}
	command := verdict["command_readback"].(map[string]any)
	if command["schema"] != "ao.command.operator-workflow-readback.v0.1" ||
		command["compatibility_gate_state"] != "ready" ||
		command["compatibility_gate_activation_authorized"] != false ||
		command["promotion_requested"] != false ||
		command["rsi_authorized"] != false {
		t.Fatalf("adoption Month 3 verdict missing Command readback: %#v", verdict)
	}
	if !strings.Contains(verdict["operator_summary"].(string), "freshness does not imply promotion") ||
		!strings.Contains(verdict["operator_summary"].(string), "RSI remains denied") {
		t.Fatalf("adoption Month 3 verdict missing operator-safe wording: %#v", verdict)
	}
}

func TestAdoptionMonth5SupportReadinessNoPromotionFixture(t *testing.T) {
	root := filepath.Join("..", "..")
	verdict := readMap(t, filepath.Join(root, "examples", "evidence", "valid", "adoption-month5-support-readiness-no-promotion.json"))
	if verdict["schema_version"] != "ao.promoter.adoption-month5-support-readiness-no-promotion.v0.1" ||
		verdict["status"] != "ready" ||
		verdict["subject"] != "adoption-month5-support-readiness" ||
		verdict["support_readiness_status"] != "fresh" ||
		verdict["compatibility_gate_state"] != "ready" ||
		verdict["compatibility_gate_active"] != false ||
		verdict["promotion_requested"] != false ||
		verdict["promotion_granted"] != false ||
		verdict["rsi_authorized"] != false ||
		verdict["external_beta_launched"] != false ||
		verdict["support_readiness_does_not_imply_promotion"] != true {
		t.Fatalf("unexpected adoption Month 5 support readiness verdict: %#v", verdict)
	}
	for _, key := range []string{
		"provider_pilot_ran",
		"release_or_publish",
		"tag_created",
		"upload_performed",
		"deployment_performed",
		"live_self_modification",
		"compatibility_gate_activation_authorized",
	} {
		if verdict[key] != false {
			t.Fatalf("adoption Month 5 verdict %s = %#v, want false", key, verdict[key])
		}
	}
	support := verdict["support_readback"].(map[string]any)
	if support["support_states"] != "fresh_stale_blocked_denied_unsupported" ||
		support["support_package"] != "install_checksum_manifest_approval_rollback_windows_operator_issue_fields" {
		t.Fatalf("adoption Month 5 verdict missing support readback: %#v", verdict)
	}
	command := verdict["command_readback"].(map[string]any)
	if command["schema"] != "ao.command.operator-workflow-readback.v0.1" ||
		command["compatibility_gate_state"] != "ready" ||
		command["compatibility_gate_activation_authorized"] != false ||
		command["promotion_requested"] != false ||
		command["rsi_authorized"] != false {
		t.Fatalf("adoption Month 5 verdict missing Command readback: %#v", verdict)
	}
	if !strings.Contains(verdict["operator_summary"].(string), "support readiness does not imply promotion") ||
		!strings.Contains(verdict["operator_summary"].(string), "RSI remains denied") {
		t.Fatalf("adoption Month 5 verdict missing operator-safe wording: %#v", verdict)
	}
}

func TestLiveMutationBoundary(t *testing.T) {
	f := newFixtureSet(t)
	paths := f.liveMutationEvidencePaths(t, false, false)
	outPath := filepath.Join(f.tmp, "live-mutation-boundary.json")
	assertRunOK(t, liveMutationBoundaryArgs(paths, outPath))
	boundary := readMap(t, outPath)
	if boundary["schema_version"] != "ao.promoter.live-mutation-boundary.v0.1" ||
		boundary["status"] != "passed" ||
		boundary["live_mutation_activation_allowed"] != true ||
		boundary["mutates_live_state"] != false ||
		boundary["mutates_repositories"] != false {
		t.Fatalf("unexpected live-mutation boundary: %#v", boundary)
	}
	if len(boundary["gate_results"].([]any)) != 7 {
		t.Fatalf("boundary should include seven gate results: %#v", boundary["gate_results"])
	}
	if boundary["current_mutation_class"] != "docs_only_single_file" ||
		boundary["next_mutation_class"] != "docs_only_multi_file" ||
		boundary["safe_to_promote_next_class"] != true {
		t.Fatalf("boundary should expose ready class promotion: %#v", boundary)
	}
	readiness, ok := boundary["class_promotion_readiness"].(map[string]any)
	if !ok {
		t.Fatalf("boundary missing class promotion readiness: %#v", boundary)
	}
	if readiness["status"] != "ready" ||
		readiness["highest_proven_live_class"] != "docs_only_single_file" ||
		readiness["current_class_live_evidence_status"] != "completed" ||
		readiness["completed_live_rehearsal"] != true ||
		readiness["rollback_proof"] != true ||
		readiness["clean_main_ci"] != true ||
		readiness["active_holds_clear"] != true {
		t.Fatalf("unexpected class promotion readiness: %#v", readiness)
	}

	multiRepoPaths := f.liveMutationEvidencePaths(t, false, false)
	for _, key := range []string{"authority", "foundry", "forge", "ao2"} {
		artifact := readMap(t, multiRepoPaths[key])
		artifact["scope"] = "multi_repo_low_risk_dry_run"
		artifact["mutation_class"] = "multi_repo_low_risk"
		artifact["current_mutation_class"] = "low_risk_code"
		artifact["next_mutation_class"] = "multi_repo_low_risk"
		artifact["safe_to_request"] = true
		artifact["safe_to_execute"] = false
		multiRepoPaths[key] = f.writeJSON("multi-repo-"+key+".json", artifact)
	}
	sentinel := readMap(t, multiRepoPaths["sentinel"])
	sentinel["mutation_class"] = "multi_repo_low_risk"
	sentinel["class_hold_verdict"] = map[string]any{"status": "clear", "mutation_class": "multi_repo_low_risk", "blockers": []any{}}
	multiRepoPaths["sentinel"] = f.writeJSON("multi-repo-sentinel.json", sentinel)
	rollback := readMap(t, multiRepoPaths["rollback"])
	rollback["mutation_class"] = "low_risk_code"
	rollback["rollback_verified"] = true
	multiRepoPaths["rollback"] = f.writeJSON("multi-repo-rollback.json", rollback)
	command := readMap(t, multiRepoPaths["command"])
	command["current_mutation_class"] = "low_risk_code"
	command["next_mutation_class"] = "multi_repo_low_risk"
	command["completed_live_rehearsal"] = map[string]any{"status": "missing", "mutation_class": "low_risk_code"}
	command["safe_to_request"] = true
	command["safe_to_execute"] = false
	multiRepoPaths["command"] = f.writeJSON("multi-repo-command-missing-low-risk-live.json", command)
	multiRepoOut := filepath.Join(f.tmp, "multi-repo-missing-low-risk-live.json")
	assertRunOK(t, liveMutationBoundaryArgs(multiRepoPaths, multiRepoOut))
	multiRepo := readMap(t, multiRepoOut)
	if multiRepo["status"] != "failed" || multiRepo["safe_to_promote_next_class"] != false {
		t.Fatalf("missing low_risk_code live evidence should deny multi_repo_low_risk promotion: %#v", multiRepo)
	}
	multiRepoReadiness := multiRepo["class_promotion_readiness"].(map[string]any)
	if multiRepoReadiness["highest_proven_live_class"] != "test_only" ||
		multiRepoReadiness["current_class_live_evidence_status"] != "missing" ||
		multiRepoReadiness["next_denied_class"] != "multi_repo_low_risk" ||
		multiRepoReadiness["next_denied_reason"] != "denied until low_risk_code completed live rehearsal evidence is recorded" ||
		!boundaryHasBlocker(multiRepo, "class_promotion_live_rehearsal") {
		t.Fatalf("multi_repo_low_risk denial readback is incomplete: %#v", multiRepoReadiness)
	}

	multiRepoReadyPaths := multiRepoPromotionFixture(t, f, nil)
	multiRepoReadyOut := filepath.Join(f.tmp, "multi-repo-ready.json")
	assertRunOK(t, liveMutationBoundaryArgs(multiRepoReadyPaths, multiRepoReadyOut))
	multiRepoReady := readMap(t, multiRepoReadyOut)
	readyPrereqs := multiRepoReady["class_promotion_readiness"].(map[string]any)["promotion_prerequisites"].(map[string]any)
	for _, key := range []string{"ordered_merge_plan", "per_repo_rollback", "ci_per_repo", "fresh_repo_state", "kill_switch"} {
		if readyPrereqs[key] != true {
			t.Fatalf("multi_repo_low_risk prerequisite %s must be true: %#v", key, readyPrereqs)
		}
	}

	for _, tc := range []struct {
		name string
		edit func(map[string]any)
		want string
	}{
		{
			name: "missing dependency",
			edit: func(command map[string]any) {
				plan := command["repo_execution_plan"].([]any)
				foundry := plan[1].(map[string]any)
				foundry["depends_on"] = []any{"ao-command"}
				foundry["merge_after"] = []any{"ao-command"}
			},
			want: "class_promotion_ordered_merge_plan",
		},
		{
			name: "stale repo state",
			edit: func(command map[string]any) {
				plan := command["repo_execution_plan"].([]any)
				foundry := plan[1].(map[string]any)
				foundry["repo_state_status"] = "stale"
				foundry["repo_state_expires_at_utc"] = "2000-01-01T00:00:00Z"
			},
			want: "class_promotion_repo_state",
		},
		{
			name: "partial rollback",
			edit: func(command map[string]any) {
				rollbacks := command["per_repo_rollback"].([]any)
				commandRollback := rollbacks[2].(map[string]any)
				commandRollback["status"] = "missing"
				commandRollback["rollback_scope"] = []any{}
			},
			want: "class_promotion_per_repo_rollback",
		},
		{
			name: "missing per repo ci",
			edit: func(command map[string]any) {
				ci := command["per_repo_ci"].([]any)
				commandCI := ci[2].(map[string]any)
				commandCI["status"] = "pending"
			},
			want: "class_promotion_per_repo_ci",
		},
		{
			name: "kill switch disarmed",
			edit: func(command map[string]any) {
				command["kill_switch_state"] = "disarmed"
			},
			want: "class_promotion_kill_switch",
		},
	} {
		t.Run("multi_repo_"+strings.ReplaceAll(tc.name, " ", "_"), func(t *testing.T) {
			paths := multiRepoPromotionFixture(t, f, tc.edit)
			out := filepath.Join(f.tmp, strings.ReplaceAll(tc.name, " ", "-")+".json")
			assertRunOK(t, liveMutationBoundaryArgs(paths, out))
			denied := readMap(t, out)
			if denied["status"] != "failed" || !boundaryHasBlocker(denied, tc.want) {
				t.Fatalf("%s should deny multi_repo_low_risk promotion with %s: %#v", tc.name, tc.want, denied)
			}
		})
	}

	holdPaths := f.liveMutationEvidencePaths(t, true, false)
	holdOut := filepath.Join(f.tmp, "live-mutation-boundary-hold.json")
	assertRunOK(t, liveMutationBoundaryArgs(holdPaths, holdOut))
	hold := readMap(t, holdOut)
	if hold["status"] != "failed" || hold["live_mutation_activation_allowed"] != false {
		t.Fatalf("Sentinel hold should block boundary: %#v", hold)
	}
	if hold["safe_to_promote_next_class"] != false {
		t.Fatalf("active Sentinel hold must deny class promotion: %#v", hold)
	}

	forbiddenPaths := f.liveMutationEvidencePaths(t, false, true)
	assertRunOK(t, liveMutationBoundaryArgs(forbiddenPaths, filepath.Join(f.tmp, "live-mutation-boundary-forbidden.json")))
	forbidden := readMap(t, filepath.Join(f.tmp, "live-mutation-boundary-forbidden.json"))
	if forbidden["status"] != "failed" {
		t.Fatalf("forbidden authority should fail boundary: %#v", forbidden)
	}

	classBlockers := []struct {
		name string
		edit func(map[string]string)
		want string
	}{
		{
			name: "missing live rehearsal",
			edit: func(paths map[string]string) {
				command := readMap(t, paths["command"])
				command["completed_live_rehearsal"] = map[string]any{"status": "missing", "mutation_class": "docs_only_single_file"}
				paths["command"] = f.writeJSON("live-command-missing-rehearsal.json", command)
			},
			want: "class_promotion_live_rehearsal",
		},
		{
			name: "missing rollback proof",
			edit: func(paths map[string]string) {
				rollback := readMap(t, paths["rollback"])
				rollback["rollback_verified"] = false
				paths["rollback"] = f.writeJSON("live-rollback-missing-proof.json", rollback)
			},
			want: "class_promotion_rollback",
		},
		{
			name: "main ci failed",
			edit: func(paths map[string]string) {
				command := readMap(t, paths["command"])
				command["clean_main_ci"] = map[string]any{"status": "failed", "branch": "main", "observed_at_utc": "2026-06-29T00:00:00Z"}
				paths["command"] = f.writeJSON("live-command-main-ci-failed.json", command)
			},
			want: "class_promotion_main_ci",
		},
	}
	for _, tc := range classBlockers {
		t.Run(tc.name, func(t *testing.T) {
			paths := f.liveMutationEvidencePaths(t, false, false)
			tc.edit(paths)
			out := filepath.Join(f.tmp, strings.ReplaceAll(tc.name, " ", "-")+".json")
			assertRunOK(t, liveMutationBoundaryArgs(paths, out))
			denied := readMap(t, out)
			if denied["status"] != "failed" || denied["safe_to_promote_next_class"] != false {
				t.Fatalf("%s should deny class promotion: %#v", tc.name, denied)
			}
			blockers := denied["blockers"].([]any)
			found := false
			for _, item := range blockers {
				if b, ok := item.(map[string]any); ok && strings.Contains(b["blocker_id"].(string), tc.want) {
					found = true
				}
			}
			if !found {
				t.Fatalf("%s missing blocker %s: %#v", tc.name, tc.want, blockers)
			}
		})
	}
}

func TestLiveDocsMutationBoundary(t *testing.T) {
	f := newFixtureSet(t)
	paths := f.liveDocsMutationEvidencePaths(t, false, false)
	outPath := filepath.Join(f.tmp, "live-docs-boundary.json")
	assertRunOK(t, liveDocsMutationBoundaryArgs(paths, outPath))
	boundary := readMap(t, outPath)
	if boundary["schema_version"] != "ao.promoter.live-docs-mutation-boundary.v0.1" ||
		boundary["status"] != "passed" ||
		boundary["first_live_class"] != "docs_only" ||
		boundary["live_docs_activation_allowed"] != true ||
		boundary["safe_to_promote_first_docs_only_live_rehearsal"] != true ||
		boundary["mutates_repositories"] != false ||
		boundary["fully_unsupervised_complex_mutation_claimed"] != false {
		t.Fatalf("unexpected live docs mutation boundary: %#v", boundary)
	}
	if len(boundary["gate_results"].([]any)) != 7 {
		t.Fatalf("docs boundary should include seven gate results: %#v", boundary["gate_results"])
	}

	holdPaths := f.liveDocsMutationEvidencePaths(t, true, false)
	holdOut := filepath.Join(f.tmp, "live-docs-boundary-hold.json")
	assertRunOK(t, liveDocsMutationBoundaryArgs(holdPaths, holdOut))
	hold := readMap(t, holdOut)
	if hold["status"] != "failed" || hold["live_docs_activation_allowed"] != false {
		t.Fatalf("Sentinel docs hold should block boundary: %#v", hold)
	}

	forbiddenPaths := f.liveDocsMutationEvidencePaths(t, false, true)
	forbiddenOut := filepath.Join(f.tmp, "live-docs-boundary-forbidden.json")
	assertRunOK(t, liveDocsMutationBoundaryArgs(forbiddenPaths, forbiddenOut))
	forbidden := readMap(t, forbiddenOut)
	if forbidden["status"] != "failed" || forbidden["safe_to_promote_first_docs_only_live_rehearsal"] != false {
		t.Fatalf("forbidden docs authority should fail boundary: %#v", forbidden)
	}
}

func TestComplexRepoPromotionVerdict(t *testing.T) {
	f := newFixtureSet(t)
	rollup := map[string]any{
		"schema_version":                      "ao.foundry.complex-repo-mutation-promotion-rollup.v0.1",
		"status":                              "ready",
		"mutation_class":                      "complex_repo_mutation",
		"safe_to_promote":                     true,
		"complex_repo_mutation_live_proven":   true,
		"highest_proven_live_class":           "complex_repo_mutation",
		"next_denied_class":                   "fully_unsupervised_complex_mutation",
		"fully_unsupervised_complex_mutation": "denied",
		"rsi":                                 "denied",
		"completed_nodes":                     12,
		"total_nodes":                         12,
		"checks": map[string]any{
			"all_nodes_completed":            true,
			"run_links_complete":             true,
			"node_gates_safe":                true,
			"no_concurrent_mutation":         true,
			"pr_ci_merge_evidence":           true,
			"rollback_evidence":              true,
			"sentinel_evidence":              true,
			"promoter_evidence":              true,
			"command_readback":               true,
			"atlas_final_workgraph_complete": true,
			"bounded_authority":              true,
			"forbidden_surfaces_clear":       true,
		},
		"blockers": []any{},
	}
	out := filepath.Join(f.tmp, "complex-promotion-verdict.json")
	assertRunOK(t, []string{"live-mutation", "complex-verdict", "--rollup", f.writeJSON("complex-rollup.ready.json", rollup), "--out", out})
	verdict := readMap(t, out)
	if verdict["schema_version"] != "ao.promoter.complex-repo-mutation-promotion-verdict.v0.1" ||
		verdict["status"] != "promoted" ||
		verdict["safe_to_promote"] != true ||
		verdict["highest_proven_live_class"] != "complex_repo_mutation" ||
		verdict["next_denied_class"] != "fully_unsupervised_complex_mutation" ||
		verdict["fully_unsupervised_complex_mutation"] != "denied" ||
		verdict["rsi"] != "denied" {
		t.Fatalf("unexpected complex promotion verdict: %#v", verdict)
	}

	rollup["status"] = "blocked"
	rollup["safe_to_promote"] = false
	rollup["complex_repo_mutation_live_proven"] = false
	rollup["highest_proven_live_class"] = "multi_repo_low_risk"
	rollup["next_denied_class"] = "complex_repo_mutation"
	rollup["first_failing_check"] = "run-link complex-docs-intake requires rollback evidence"
	rollup["blockers"] = []any{"run-link complex-docs-intake requires rollback evidence"}
	blockedOut := filepath.Join(f.tmp, "complex-promotion-verdict-blocked.json")
	assertRunOK(t, []string{"live-mutation", "complex-verdict", "--rollup", f.writeJSON("complex-rollup.blocked.json", rollup), "--out", blockedOut})
	blocked := readMap(t, blockedOut)
	if blocked["status"] != "denied" ||
		blocked["safe_to_promote"] != false ||
		blocked["highest_proven_live_class"] != "multi_repo_low_risk" ||
		blocked["next_denied_class"] != "complex_repo_mutation" ||
		!boundaryHasBlocker(blocked, "complex_repo_mutation_promotion_rollup") {
		t.Fatalf("blocked rollup must produce denied complex promotion verdict: %#v", blocked)
	}
}

func liveMutationBoundaryArgs(paths map[string]string, out string) []string {
	return []string{
		"live-mutation", "boundary",
		"--authority", paths["authority"],
		"--foundry-request", paths["foundry"],
		"--forge-plan", paths["forge"],
		"--ao2-packet", paths["ao2"],
		"--sentinel-hold", paths["sentinel"],
		"--rollback", paths["rollback"],
		"--command-status", paths["command"],
		"--out", out,
	}
}

func liveDocsMutationBoundaryArgs(paths map[string]string, out string) []string {
	return []string{
		"live-mutation", "docs-boundary",
		"--approval-ticket", paths["approval"],
		"--foundry-gate", paths["foundry"],
		"--forge-guard", paths["forge"],
		"--ao2-packet", paths["ao2"],
		"--sentinel-verdict", paths["sentinel"],
		"--rollback", paths["rollback"],
		"--command-readback", paths["command"],
		"--out", out,
	}
}

func TestCheckedInExamplesAreCovered(t *testing.T) {
	root := filepath.Join("..", "..")

	assertRunOK(t, []string{"candidate", "validate", "--candidate", filepath.Join(root, "examples/candidates/valid/ao-foundry-candidate.json")})
	assertRunOK(t, []string{"packet", "validate", "--packet", filepath.Join(root, "examples/packets/valid/ao-promoter-v0.1.json")})
	assertRunOK(t, []string{"live-mutation", "boundary", "--authority", filepath.Join(root, "examples/live-mutation/valid/covenant-authority.approved.json"), "--foundry-request", filepath.Join(root, "examples/live-mutation/valid/foundry-request.ready.json"), "--forge-plan", filepath.Join(root, "examples/live-mutation/valid/forge-plan.ready.json"), "--ao2-packet", filepath.Join(root, "examples/live-mutation/valid/ao2-packet.ready.json"), "--sentinel-hold", filepath.Join(root, "examples/live-mutation/valid/sentinel-hold.clear.json"), "--rollback", filepath.Join(root, "examples/live-mutation/valid/rollback-rehearsal.ready.json"), "--command-status", filepath.Join(root, "examples/live-mutation/valid/command-status.ready.json"), "--out", filepath.Join(root, "tmp/checked-in-live-mutation-boundary.json")})
	testOnlyBoundaryPath := filepath.Join(root, "tmp/checked-in-live-mutation-test-only-boundary.json")
	assertRunOK(t, []string{"live-mutation", "boundary", "--authority", filepath.Join(root, "examples/live-mutation/valid/covenant-authority.docs-multi-approved.json"), "--foundry-request", filepath.Join(root, "examples/live-mutation/valid/foundry-request.docs-multi-ready.json"), "--forge-plan", filepath.Join(root, "examples/live-mutation/valid/forge-plan.docs-multi-ready.json"), "--ao2-packet", filepath.Join(root, "examples/live-mutation/valid/ao2-packet.test-only-ready.json"), "--sentinel-hold", filepath.Join(root, "examples/live-mutation/valid/sentinel-hold.test-only-clear.json"), "--rollback", filepath.Join(root, "examples/live-mutation/valid/rollback-rehearsal.docs-multi-ready.json"), "--command-status", filepath.Join(root, "examples/live-mutation/valid/command-status.test-only-ready.json"), "--out", testOnlyBoundaryPath})
	testOnlyBoundary := readMap(t, testOnlyBoundaryPath)
	if testOnlyBoundary["status"] != "passed" ||
		testOnlyBoundary["current_mutation_class"] != "docs_only_multi_file" ||
		testOnlyBoundary["next_mutation_class"] != "test_only" ||
		testOnlyBoundary["safe_to_promote_next_class"] != true ||
		testOnlyBoundary["dry_run_only"] != true {
		t.Fatalf("checked-in test_only promotion boundary should pass as dry-run readiness: %#v", testOnlyBoundary)
	}
	lowRiskBoundaryPath := filepath.Join(root, "tmp/checked-in-live-mutation-low-risk-code-boundary.json")
	assertRunOK(t, []string{"live-mutation", "boundary", "--authority", filepath.Join(root, "examples/live-mutation/valid/covenant-authority.low-risk-code-approved.json"), "--foundry-request", filepath.Join(root, "examples/live-mutation/valid/foundry-request.low-risk-code-ready.json"), "--forge-plan", filepath.Join(root, "examples/live-mutation/valid/forge-plan.low-risk-code-ready.json"), "--ao2-packet", filepath.Join(root, "examples/live-mutation/valid/ao2-packet.low-risk-code-ready.json"), "--sentinel-hold", filepath.Join(root, "examples/live-mutation/valid/sentinel-hold.low-risk-code-clear.json"), "--rollback", filepath.Join(root, "examples/live-mutation/valid/rollback-rehearsal.test-only-ready.json"), "--command-status", filepath.Join(root, "examples/live-mutation/valid/command-status.low-risk-code-ready.json"), "--out", lowRiskBoundaryPath})
	lowRiskBoundary := readMap(t, lowRiskBoundaryPath)
	if lowRiskBoundary["status"] != "passed" ||
		lowRiskBoundary["current_mutation_class"] != "test_only" ||
		lowRiskBoundary["next_mutation_class"] != "low_risk_code" ||
		lowRiskBoundary["safe_to_promote_next_class"] != true ||
		lowRiskBoundary["dry_run_only"] != true {
		t.Fatalf("checked-in low_risk_code promotion boundary should pass as dry-run readiness: %#v", lowRiskBoundary)
	}
	lowRiskReadiness := lowRiskBoundary["class_promotion_readiness"].(map[string]any)
	prereqs, ok := lowRiskReadiness["promotion_prerequisites"].(map[string]any)
	if !ok {
		t.Fatalf("low_risk_code readiness must expose promotion_prerequisites: %#v", lowRiskReadiness)
	}
	for _, key := range []string{
		"successful_test_only_live_evidence",
		"rollback_fixture",
		"sentinel_clear_verdict",
		"clean_main_ci",
		"exact_covenant_class_ticket",
		"command_readback",
	} {
		if prereqs[key] != true {
			t.Fatalf("low_risk_code prerequisite %s must be true: %#v", key, prereqs)
		}
	}
	wrongTicketBoundaryPath := filepath.Join(root, "tmp/checked-in-live-mutation-low-risk-code-wrong-ticket-boundary.json")
	assertRunOK(t, []string{"live-mutation", "boundary", "--authority", filepath.Join(root, "examples/live-mutation/invalid/covenant-authority.wrong-class-for-low-risk.json"), "--foundry-request", filepath.Join(root, "examples/live-mutation/valid/foundry-request.low-risk-code-ready.json"), "--forge-plan", filepath.Join(root, "examples/live-mutation/valid/forge-plan.low-risk-code-ready.json"), "--ao2-packet", filepath.Join(root, "examples/live-mutation/valid/ao2-packet.low-risk-code-ready.json"), "--sentinel-hold", filepath.Join(root, "examples/live-mutation/valid/sentinel-hold.low-risk-code-clear.json"), "--rollback", filepath.Join(root, "examples/live-mutation/valid/rollback-rehearsal.test-only-ready.json"), "--command-status", filepath.Join(root, "examples/live-mutation/valid/command-status.low-risk-code-ready.json"), "--out", wrongTicketBoundaryPath})
	wrongTicketBoundary := readMap(t, wrongTicketBoundaryPath)
	if wrongTicketBoundary["status"] != "failed" ||
		wrongTicketBoundary["safe_to_promote_next_class"] != false ||
		!boundaryHasBlocker(wrongTicketBoundary, "class_promotion_covenant_ticket") {
		t.Fatalf("wrong Covenant class ticket must deny low_risk_code promotion: %#v", wrongTicketBoundary)
	}
	checkedLowRiskPrereqs := readMap(t, filepath.Join(root, "examples/live-mutation/valid/live-mutation-boundary.low-risk-code-prereqs.passed.json"))
	if checkedLowRiskPrereqs["status"] != "passed" ||
		checkedLowRiskPrereqs["safe_to_promote_next_class"] != true {
		t.Fatalf("checked low_risk_code prerequisite fixture drifted: %#v", checkedLowRiskPrereqs)
	}
	checkedInvalidLowRiskPrereqs := readMap(t, filepath.Join(root, "examples/live-mutation/invalid/live-mutation-boundary.low-risk-code-wrong-ticket.failed.json"))
	if checkedInvalidLowRiskPrereqs["status"] != "failed" ||
		!boundaryHasBlocker(checkedInvalidLowRiskPrereqs, "class_promotion_covenant_ticket") {
		t.Fatalf("checked low_risk_code wrong-ticket fixture drifted: %#v", checkedInvalidLowRiskPrereqs)
	}
	assertRunOK(t, []string{"live-mutation", "docs-boundary", "--approval-ticket", filepath.Join(root, "examples/live-docs-mutation/valid/approval-ticket.approved.json"), "--foundry-gate", filepath.Join(root, "examples/live-docs-mutation/valid/foundry-approval-gate.ready.json"), "--forge-guard", filepath.Join(root, "examples/live-docs-mutation/valid/forge-guard.ready.json"), "--ao2-packet", filepath.Join(root, "examples/live-docs-mutation/valid/ao2-docs-packet.ready.json"), "--sentinel-verdict", filepath.Join(root, "examples/live-docs-mutation/valid/sentinel-verdict.clear.json"), "--rollback", filepath.Join(root, "examples/live-docs-mutation/valid/rollback-execution.ready.json"), "--command-readback", filepath.Join(root, "examples/live-docs-mutation/valid/command-readback.ready.json"), "--out", filepath.Join(root, "tmp/checked-in-live-docs-boundary.json")})
	invalidBoundaryPath := filepath.Join(root, "tmp/checked-in-invalid-live-mutation-boundary.json")
	assertRunOK(t, []string{"live-mutation", "boundary", "--authority", filepath.Join(root, "examples/live-mutation/valid/covenant-authority.approved.json"), "--foundry-request", filepath.Join(root, "examples/live-mutation/valid/foundry-request.ready.json"), "--forge-plan", filepath.Join(root, "examples/live-mutation/valid/forge-plan.ready.json"), "--ao2-packet", filepath.Join(root, "examples/live-mutation/invalid/ao2-packet.forbidden-authority.json"), "--sentinel-hold", filepath.Join(root, "examples/live-mutation/valid/sentinel-hold.clear.json"), "--rollback", filepath.Join(root, "examples/live-mutation/valid/rollback-rehearsal.ready.json"), "--command-status", filepath.Join(root, "examples/live-mutation/valid/command-status.ready.json"), "--out", invalidBoundaryPath})
	if invalidBoundary := readMap(t, invalidBoundaryPath); invalidBoundary["status"] != "failed" {
		t.Fatalf("forbidden live-mutation authority should emit failed boundary: %#v", invalidBoundary)
	}
	for _, tc := range []struct {
		name  string
		args  []string
		block string
	}{
		{
			name:  "missing live rehearsal",
			args:  []string{"live-mutation", "boundary", "--authority", filepath.Join(root, "examples/live-mutation/valid/covenant-authority.approved.json"), "--foundry-request", filepath.Join(root, "examples/live-mutation/valid/foundry-request.ready.json"), "--forge-plan", filepath.Join(root, "examples/live-mutation/valid/forge-plan.ready.json"), "--ao2-packet", filepath.Join(root, "examples/live-mutation/valid/ao2-packet.ready.json"), "--sentinel-hold", filepath.Join(root, "examples/live-mutation/valid/sentinel-hold.clear.json"), "--rollback", filepath.Join(root, "examples/live-mutation/valid/rollback-rehearsal.ready.json"), "--command-status", filepath.Join(root, "examples/live-mutation/invalid/command-status.missing-live-rehearsal.json")},
			block: "class_promotion_live_rehearsal",
		},
		{
			name:  "main ci failed",
			args:  []string{"live-mutation", "boundary", "--authority", filepath.Join(root, "examples/live-mutation/valid/covenant-authority.approved.json"), "--foundry-request", filepath.Join(root, "examples/live-mutation/valid/foundry-request.ready.json"), "--forge-plan", filepath.Join(root, "examples/live-mutation/valid/forge-plan.ready.json"), "--ao2-packet", filepath.Join(root, "examples/live-mutation/valid/ao2-packet.ready.json"), "--sentinel-hold", filepath.Join(root, "examples/live-mutation/valid/sentinel-hold.clear.json"), "--rollback", filepath.Join(root, "examples/live-mutation/valid/rollback-rehearsal.ready.json"), "--command-status", filepath.Join(root, "examples/live-mutation/invalid/command-status.main-ci-failed.json")},
			block: "class_promotion_main_ci",
		},
		{
			name:  "rollback missing proof",
			args:  []string{"live-mutation", "boundary", "--authority", filepath.Join(root, "examples/live-mutation/valid/covenant-authority.approved.json"), "--foundry-request", filepath.Join(root, "examples/live-mutation/valid/foundry-request.ready.json"), "--forge-plan", filepath.Join(root, "examples/live-mutation/valid/forge-plan.ready.json"), "--ao2-packet", filepath.Join(root, "examples/live-mutation/valid/ao2-packet.ready.json"), "--sentinel-hold", filepath.Join(root, "examples/live-mutation/valid/sentinel-hold.clear.json"), "--rollback", filepath.Join(root, "examples/live-mutation/invalid/rollback-rehearsal.missing-proof.json"), "--command-status", filepath.Join(root, "examples/live-mutation/valid/command-status.ready.json")},
			block: "class_promotion_rollback",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := filepath.Join(root, "tmp", strings.ReplaceAll(tc.name, " ", "-")+".json")
			args := append(append([]string{}, tc.args...), "--out", out)
			assertRunOK(t, args)
			boundary := readMap(t, out)
			if boundary["status"] != "failed" || boundary["safe_to_promote_next_class"] != false {
				t.Fatalf("%s should deny class promotion: %#v", tc.name, boundary)
			}
			if !boundaryHasBlocker(boundary, tc.block) {
				t.Fatalf("%s missing blocker %s: %#v", tc.name, tc.block, boundary["blockers"])
			}
		})
	}
	invalidDocsBoundaryPath := filepath.Join(root, "tmp/checked-in-invalid-live-docs-boundary.json")
	assertRunOK(t, []string{"live-mutation", "docs-boundary", "--approval-ticket", filepath.Join(root, "examples/live-docs-mutation/valid/approval-ticket.approved.json"), "--foundry-gate", filepath.Join(root, "examples/live-docs-mutation/valid/foundry-approval-gate.ready.json"), "--forge-guard", filepath.Join(root, "examples/live-docs-mutation/valid/forge-guard.ready.json"), "--ao2-packet", filepath.Join(root, "examples/live-docs-mutation/invalid/ao2-docs-packet.forbidden-authority.json"), "--sentinel-verdict", filepath.Join(root, "examples/live-docs-mutation/valid/sentinel-verdict.clear.json"), "--rollback", filepath.Join(root, "examples/live-docs-mutation/valid/rollback-execution.ready.json"), "--command-readback", filepath.Join(root, "examples/live-docs-mutation/valid/command-readback.ready.json"), "--out", invalidDocsBoundaryPath})
	if invalidDocsBoundary := readMap(t, invalidDocsBoundaryPath); invalidDocsBoundary["status"] != "failed" {
		t.Fatalf("forbidden live-docs authority should emit failed boundary: %#v", invalidDocsBoundary)
	}

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

func (f fixtureSet) liveMutationEvidencePaths(t *testing.T, sentinelHold bool, forbiddenAuthority bool) map[string]string {
	t.Helper()
	authority := map[string]any{
		"schema_version":       "covenant.live-mutation-authority.v1",
		"status":               "approved",
		"mode":                 "dry_run_only",
		"scope":                "docs_only_fixture",
		"mutation_class":       "docs_only_single_file",
		"mutates_live_state":   false,
		"mutates_repositories": false,
	}
	foundry := map[string]any{
		"schema_version":       "ao.foundry.live-mutation-request.v0.1",
		"status":               "ready",
		"mode":                 "dry_run_only",
		"mutation_class":       "docs_only_single_file",
		"mutates_live_state":   false,
		"mutates_repositories": false,
	}
	forge := map[string]any{
		"schema_version":       "ao.forge.live-mutation-dry-run-plan.v0.1",
		"status":               "ready",
		"mode":                 "dry_run_only",
		"mutation_class":       "docs_only_single_file",
		"mutates_live_state":   false,
		"mutates_repositories": false,
	}
	ao2 := map[string]any{
		"schema_version":       "ao2.live-mutation-dry-run-packet.v1",
		"status":               "ready",
		"mode":                 "dry_run_only",
		"mutation_class":       "docs_only_single_file",
		"mutates_live_state":   false,
		"mutates_repositories": false,
	}
	sentinel := map[string]any{
		"schema_version":         "ao.sentinel.live-mutation-hold.v0.1",
		"status":                 "clear",
		"mutation_class":         "docs_only_single_file",
		"class_hold_verdict":     map[string]any{"status": "clear", "mutation_class": "docs_only_single_file", "blockers": []any{}},
		"hold_required":          false,
		"promoter_hold_required": false,
		"mutates_live_state":     false,
		"mutates_repositories":   false,
	}
	if sentinelHold {
		sentinel["status"] = "hold"
		sentinel["hold_required"] = true
		sentinel["promoter_hold_required"] = true
	}
	rollback := map[string]any{
		"schema_version":       "ao.foundry.live-mutation-rollback-rehearsal.v0.1",
		"status":               "ready",
		"mode":                 "dry_run_only",
		"mutation_class":       "docs_only_single_file",
		"rollback_verified":    true,
		"mutates_live_state":   false,
		"mutates_repositories": false,
	}
	command := map[string]any{
		"schema_version":         "ao.command.live-mutation-status.v0.1",
		"status":                 "ready",
		"current_mutation_class": "docs_only_single_file",
		"next_mutation_class":    "docs_only_multi_file",
		"completed_live_rehearsal": map[string]any{
			"status":         "completed",
			"mutation_class": "docs_only_single_file",
			"evidence_ref":   "pr://docs-only-single-file",
		},
		"clean_main_ci": map[string]any{
			"status":          "passed",
			"branch":          "main",
			"observed_at_utc": "2026-06-29T00:00:00Z",
		},
		"active_holds":         []any{},
		"kill_switch_state":    "armed",
		"operator_mode":        "read_only",
		"mutates_live_state":   false,
		"mutates_repositories": false,
	}
	if forbiddenAuthority {
		ao2["mutates_repositories"] = true
	}
	return map[string]string{
		"authority": f.writeJSON("live-authority.json", authority),
		"foundry":   f.writeJSON("live-foundry.json", foundry),
		"forge":     f.writeJSON("live-forge.json", forge),
		"ao2":       f.writeJSON("live-ao2.json", ao2),
		"sentinel":  f.writeJSON("live-sentinel.json", sentinel),
		"rollback":  f.writeJSON("live-rollback.json", rollback),
		"command":   f.writeJSON("live-command.json", command),
	}
}

func multiRepoPromotionFixture(t *testing.T, f fixtureSet, editCommand func(map[string]any)) map[string]string {
	t.Helper()
	paths := f.liveMutationEvidencePaths(t, false, false)
	for _, key := range []string{"authority", "foundry", "forge", "ao2"} {
		artifact := readMap(t, paths[key])
		artifact["scope"] = "multi_repo_low_risk_dry_run"
		artifact["mutation_class"] = "multi_repo_low_risk"
		artifact["current_mutation_class"] = "low_risk_code"
		artifact["next_mutation_class"] = "multi_repo_low_risk"
		artifact["safe_to_request"] = true
		artifact["safe_to_execute"] = false
		paths[key] = f.writeJSON("multi-repo-ready-"+key+".json", artifact)
	}
	sentinel := readMap(t, paths["sentinel"])
	sentinel["mutation_class"] = "multi_repo_low_risk"
	sentinel["class_hold_verdict"] = map[string]any{
		"status":                       "clear",
		"mutation_class":               "multi_repo_low_risk",
		"multi_repo_dependency_status": "passed",
		"per_repo_rollback_status":     "ready",
		"per_repo_ci_status":           "passed",
		"repo_state_status":            "fresh",
		"blockers":                     []any{},
	}
	paths["sentinel"] = f.writeJSON("multi-repo-ready-sentinel.json", sentinel)
	rollback := readMap(t, paths["rollback"])
	rollback["mutation_class"] = "low_risk_code"
	rollback["rollback_verified"] = true
	paths["rollback"] = f.writeJSON("multi-repo-ready-rollback.json", rollback)
	command := readMap(t, paths["command"])
	command["current_mutation_class"] = "low_risk_code"
	command["next_mutation_class"] = "multi_repo_low_risk"
	command["completed_live_rehearsal"] = map[string]any{
		"status":         "completed",
		"mutation_class": "low_risk_code",
		"evidence_ref":   "pr://ao-atlas/low-risk-code",
	}
	command["safe_to_request"] = true
	command["safe_to_execute"] = false
	command["repo_execution_plan"] = []any{
		map[string]any{"repo": "ao-atlas", "order": 1, "planned_pr": "dry-run-pr:ao-atlas", "status": "ready", "depends_on": []any{}, "merge_after": []any{}, "rollback_status": "ready", "ci_status": "passed", "repo_state_status": "clean_synced", "repo_state_expires_at_utc": "2999-01-01T00:00:00Z"},
		map[string]any{"repo": "ao-foundry", "order": 2, "planned_pr": "dry-run-pr:ao-foundry", "status": "ready", "depends_on": []any{"ao-atlas"}, "merge_after": []any{"ao-atlas"}, "rollback_status": "ready", "ci_status": "passed", "repo_state_status": "clean_synced", "repo_state_expires_at_utc": "2999-01-01T00:00:00Z"},
		map[string]any{"repo": "ao-command", "order": 3, "planned_pr": "dry-run-pr:ao-command", "status": "ready", "depends_on": []any{"ao-foundry"}, "merge_after": []any{"ao-foundry"}, "rollback_status": "ready", "ci_status": "passed", "repo_state_status": "clean_synced", "repo_state_expires_at_utc": "2999-01-01T00:00:00Z"},
	}
	command["per_repo_rollback"] = []any{
		map[string]any{"repo": "ao-atlas", "status": "ready", "rollback_scope": []any{"repo:ao-atlas:internal/**"}},
		map[string]any{"repo": "ao-foundry", "status": "ready", "rollback_scope": []any{"repo:ao-foundry:internal/**"}},
		map[string]any{"repo": "ao-command", "status": "ready", "rollback_scope": []any{"repo:ao-command:internal/**"}},
	}
	command["per_repo_ci"] = []any{
		map[string]any{"repo": "ao-atlas", "status": "passed", "required": true},
		map[string]any{"repo": "ao-foundry", "status": "passed", "required": true},
		map[string]any{"repo": "ao-command", "status": "passed", "required": true},
	}
	if editCommand != nil {
		editCommand(command)
	}
	paths["command"] = f.writeJSON("multi-repo-ready-command.json", command)
	return paths
}

func (f fixtureSet) liveDocsMutationEvidencePaths(t *testing.T, sentinelHold bool, forbiddenAuthority bool) map[string]string {
	t.Helper()
	approval := map[string]any{
		"schema_version":       "covenant.live-docs-approval-ticket.v1",
		"status":               "approved",
		"change_class":         "docs_only",
		"scope":                "docs_only",
		"approver":             "operator-fixture",
		"consumed":             false,
		"mutates_live_state":   false,
		"mutates_repositories": false,
	}
	foundry := map[string]any{
		"schema_version":       "ao.foundry.live-docs-approval-gate.v0.1",
		"status":               "ready",
		"safe_to_execute":      true,
		"mutates_live_state":   false,
		"mutates_repositories": false,
	}
	forge := map[string]any{
		"schema_version":                  "ao.forge.live-docs-execution-guard.v0.1",
		"status":                          "ready",
		"docs_only_allowlist_enforced":    true,
		"rollback_plan_required":          true,
		"mutates_live_state":              false,
		"mutates_repositories":            false,
		"schedules_work":                  false,
		"executes_work":                   false,
		"approves_work":                   false,
		"provider_calls_allowed":          false,
		"release_or_publish_allowed":      false,
		"broad_live_mutation_allowed":     false,
		"ungated_live_mutation_requested": false,
	}
	ao2 := map[string]any{
		"schema_version":             "ao2.docs-only-patch-packet.v1",
		"status":                     "ready",
		"dry_run_apply":              true,
		"rollback_patch_present":     true,
		"mutates_live_state":         false,
		"mutates_repositories":       false,
		"schedules_work":             false,
		"executes_work":              false,
		"approves_work":              false,
		"provider_calls_allowed":     false,
		"release_or_publish_allowed": false,
	}
	sentinel := map[string]any{
		"schema_version":         "ao.sentinel.live-docs-mutation-hold.v0.1",
		"status":                 "clear",
		"hold_required":          false,
		"promoter_hold_required": false,
		"mutates_live_state":     false,
		"mutates_repositories":   false,
	}
	if sentinelHold {
		sentinel["status"] = "hold"
		sentinel["hold_required"] = true
		sentinel["promoter_hold_required"] = true
	}
	rollback := map[string]any{
		"schema_version":       "ao.foundry.live-docs-rollback-execution-rehearsal.v0.1",
		"status":               "ready",
		"rollback_verified":    true,
		"mutates_live_state":   false,
		"mutates_repositories": false,
	}
	command := map[string]any{
		"schema_version":       "ao.command.live-docs-mutation-status.v0.1",
		"status":               "ready",
		"kill_switch_state":    "armed",
		"operator_mode":        "read_only",
		"mutates_live_state":   false,
		"mutates_repositories": false,
	}
	if forbiddenAuthority {
		ao2["mutates_repositories"] = true
	}
	return map[string]string{
		"approval": f.writeJSON("live-docs-approval.json", approval),
		"foundry":  f.writeJSON("live-docs-foundry.json", foundry),
		"forge":    f.writeJSON("live-docs-forge.json", forge),
		"ao2":      f.writeJSON("live-docs-ao2.json", ao2),
		"sentinel": f.writeJSON("live-docs-sentinel.json", sentinel),
		"rollback": f.writeJSON("live-docs-rollback.json", rollback),
		"command":  f.writeJSON("live-docs-command.json", command),
	}
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

func boundaryHasBlocker(boundary map[string]any, marker string) bool {
	for _, item := range asAnySlice(boundary["blockers"]) {
		if blocker, ok := item.(map[string]any); ok && strings.Contains(stringField(blocker, "blocker_id"), marker) {
			return true
		}
	}
	return false
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
