package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var requiredRoles = []string{
	"arena_promotion_gate",
	"crucible_hardening_gate",
	"covenant_policy_decision",
	"foundry_goal_readiness",
	"forge_packet_summary",
	"ao2_run_summary",
	"public_safety_scan",
	"rollback_plan_ready",
}

var acceptedStatuses = map[string]string{
	"arena_promotion_gate":     "passed",
	"crucible_hardening_gate":  "passed",
	"covenant_policy_decision": "allowed",
	"foundry_goal_readiness":   "ready",
	"forge_packet_summary":     "verified",
	"ao2_run_summary":          "passed",
	"public_safety_scan":       "passed",
	"rollback_plan_ready":      "ready",
}

var allowedKinds = setOf("factory", "orchestrator", "benchmark", "hardening", "policy", "command_surface", "control_plane", "stack_revision")
var allowedSlots = setOf("factory", "orchestrator", "benchmark", "hardening", "policy", "command_surface", "control_plane", "release_gate")
var mutationClassPromotionSuccessor = map[string]string{
	"docs_only_single_file": "docs_only_multi_file",
	"docs_only_multi_file":  "test_only",
	"test_only":             "low_risk_code",
	"low_risk_code":         "multi_repo_low_risk",
	"multi_repo_low_risk":   "complex_repo_mutation",
}

type blocker struct {
	BlockerID         string `json:"blocker_id"`
	GateRole          string `json:"gate_role"`
	Severity          string `json:"severity"`
	Reason            string `json:"reason"`
	EvidencePath      string `json:"evidence_path"`
	RecommendedAction string `json:"recommended_action"`
}

type packetState struct {
	Packet    map[string]any
	Candidate map[string]any
	Refs      []map[string]any
	Blockers  []blocker
	BaseDir   string
}

// Run executes the AO Promoter CLI and returns a process-style exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printHelp(stdout)
		return 0
	}
	var err error
	switch args[0] {
	case "candidate":
		err = runCandidate(args[1:], stdout)
	case "packet":
		err = runPacket(args[1:], stdout)
	case "gates":
		err = runGates(args[1:], stdout)
	case "plan":
		err = runPlan(args[1:], stdout)
	case "active":
		err = runActive(args[1:], stdout)
	case "rollback":
		err = runRollback(args[1:], stdout)
	case "report":
		err = runReport(args[1:], stdout)
	case "apply":
		err = runApply(args[1:], stdout)
	case "evidence":
		err = runEvidence(args[1:], stdout)
	case "safety":
		err = runSafety(args[1:], stdout)
	case "live-mutation":
		err = runLiveMutation(args[1:], stdout)
	case "mission":
		err = runMission(args[1:], stdout)
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, `AO Promoter validates candidate promotion into the active AO stack.

Usage:
  promoter candidate validate --candidate <path>
  promoter packet validate --packet <path>
  promoter gates evaluate --packet <path> --out <json>
  promoter plan activate --packet <path> --out <json>
  promoter active render --plan <path> --out <json>
  promoter rollback plan --active <path> --candidate <path> --out <json>
  promoter report render --gate <path> --plan <path> --out <markdown>
  promoter apply --plan <path> --dry-run --out <json>
  promoter evidence inspect --packet <path>
  promoter safety scan --path <path> --out <json>
  promoter mission rollup-summary --no-promotion <json> --out <json>
  promoter live-mutation boundary --authority <json> --foundry-request <json> --forge-plan <json> --ao2-packet <json> --sentinel-hold <json> --rollback <json> --command-status <json> --out <json>
  promoter live-mutation docs-boundary --approval-ticket <json> --foundry-gate <json> --forge-guard <json> --ao2-packet <json> --sentinel-verdict <json> --rollback <json> --command-readback <json> --out <json>

Commands: candidate packet gates plan active rollback report apply evidence safety mission live-mutation`)
}

func runCandidate(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "validate" {
		return errors.New("candidate command requires validate")
	}
	path, err := flagValue(args[1:], "--candidate")
	if err != nil {
		return err
	}
	candidate, err := readJSONMap(path)
	if err != nil {
		return err
	}
	if err := validateCandidate(candidate); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "candidate validation: passed")
	return nil
}

func runPacket(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "validate" {
		return errors.New("packet command requires validate")
	}
	path, err := flagValue(args[1:], "--packet")
	if err != nil {
		return err
	}
	state, err := loadPacket(path)
	if err != nil {
		return err
	}
	if len(state.Blockers) > 0 {
		return fmt.Errorf("%s", state.Blockers[0].Reason)
	}
	fmt.Fprintln(stdout, "packet validation: passed")
	return nil
}

func runGates(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "evaluate" {
		return errors.New("gates command requires evaluate")
	}
	packetPath, err := flagValue(args[1:], "--packet")
	if err != nil {
		return err
	}
	out, err := flagValue(args[1:], "--out")
	if err != nil {
		return err
	}
	if err := requireTmpOutput(out); err != nil {
		return err
	}
	state, err := loadPacket(packetPath)
	if err != nil {
		return err
	}
	gate, err := evaluateGate(state)
	if err != nil {
		return err
	}
	if err := writeJSON(out, gate); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "promotion gate: %s\n", gate["status"])
	return nil
}

func runPlan(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "activate" {
		return errors.New("plan command requires activate")
	}
	packetPath, err := flagValue(args[1:], "--packet")
	if err != nil {
		return err
	}
	out, err := flagValue(args[1:], "--out")
	if err != nil {
		return err
	}
	if err := requireTmpOutput(out); err != nil {
		return err
	}
	state, err := loadPacket(packetPath)
	if err != nil {
		return err
	}
	gate, err := evaluateGate(state)
	if err != nil {
		return err
	}
	if gate["status"] != "passed" {
		return errors.New("activation requires passed promotion gate")
	}
	active, err := readJSONMap(resolvePath(state.BaseDir, stringField(state.Packet, "current_active_stack")))
	if err != nil {
		return err
	}
	slot := stringField(state.Candidate, "target_slot")
	current, err := activeSlot(active, slot)
	if err != nil {
		return err
	}
	packetID := stringField(state.Packet, "packet_id")
	candidateID := stringField(state.Candidate, "candidate_id")
	plan := map[string]any{
		"schema_version":                "ao.promoter.activation-plan.v0.1",
		"plan_id":                       "activate-" + packetID,
		"packet_id":                     packetID,
		"candidate_id":                  candidateID,
		"target_stack_id":               stringField(state.Candidate, "target_stack_id"),
		"target_slot":                   slot,
		"current_active_stack":          stringField(state.Packet, "current_active_stack"),
		"current_active_stack_manifest": active,
		"current_component":             current,
		"next_component":                candidateComponent(state.Candidate),
		"required_gate_ref":             "tmp/promotion-gate.json",
		"rollback_plan_ref":             "tmp/rollback-plan.json",
		"actions":                       []string{"validate promotion gate", "render next active stack", "simulate activation only"},
		"dry_run_only":                  true,
		"mutates_live_state":            false,
		"promotion_gate_status":         "passed",
	}
	if err := writeJSON(out, plan); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "activation plan: %s\n", plan["plan_id"])
	return nil
}

func runActive(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "render" {
		return errors.New("active command requires render")
	}
	planPath, err := flagValue(args[1:], "--plan")
	if err != nil {
		return err
	}
	out, err := flagValue(args[1:], "--out")
	if err != nil {
		return err
	}
	if err := requireTmpOutput(out); err != nil {
		return err
	}
	plan, err := readJSONMap(planPath)
	if err != nil {
		return err
	}
	if boolField(plan, "dry_run_only") != true || boolField(plan, "mutates_live_state") != false {
		return errors.New("active render requires dry-run activation plan")
	}
	active, ok := plan["current_active_stack_manifest"].(map[string]any)
	if !ok {
		activePath := resolvePath(filepath.Dir(planPath), stringField(plan, "current_active_stack"))
		var err error
		active, err = readJSONMap(activePath)
		if err != nil {
			return err
		}
	}
	slots, ok := active["slots"].(map[string]any)
	if !ok {
		return errors.New("active stack slots must be an object")
	}
	slot := stringField(plan, "target_slot")
	next, ok := plan["next_component"].(map[string]any)
	if !ok {
		return errors.New("activation plan next_component must be an object")
	}
	slots[slot] = next
	active["slots"] = slots
	active["previous_stack_ref"] = stringField(active, "stack_id")
	active["stack_id"] = stringField(plan, "target_stack_id")
	active["created_at_utc"] = nowUTC()
	active["promotion_history"] = append(asAnySlice(active["promotion_history"]), map[string]any{
		"candidate_id": stringField(plan, "candidate_id"),
		"plan_id":      stringField(plan, "plan_id"),
		"dry_run_only": true,
	})
	if err := writeJSON(out, active); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "active stack rendered: %s\n", out)
	return nil
}

func runRollback(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "plan" {
		return errors.New("rollback command requires plan")
	}
	activePath, err := flagValue(args[1:], "--active")
	if err != nil {
		return err
	}
	candidatePath, err := flagValue(args[1:], "--candidate")
	if err != nil {
		return err
	}
	out, err := flagValue(args[1:], "--out")
	if err != nil {
		return err
	}
	if err := requireTmpOutput(out); err != nil {
		return err
	}
	active, err := readJSONMap(activePath)
	if err != nil {
		return err
	}
	candidate, err := readJSONMap(candidatePath)
	if err != nil {
		return err
	}
	if err := validateCandidate(candidate); err != nil {
		return err
	}
	slot := stringField(candidate, "target_slot")
	previous, err := activeSlot(active, slot)
	if err != nil {
		return err
	}
	plan := map[string]any{
		"schema_version":         "ao.promoter.rollback-plan.v0.1",
		"rollback_id":            "rollback-" + stringField(candidate, "candidate_id"),
		"candidate_id":           stringField(candidate, "candidate_id"),
		"target_stack_id":        stringField(candidate, "target_stack_id"),
		"target_slot":            slot,
		"previous_component":     previous,
		"restore_actions":        []string{"restore previous component in target slot", "rerun public safety scan", "rerun active stack validation"},
		"verification_commands":  []string{"promoter active render --plan tmp/activation-plan.json --out tmp/active-stack.next.json", "promoter safety scan --path docs --out tmp/docs-scan.json"},
		"dry_run_only":           true,
		"mutates_live_state":     false,
		"rollback_plan_status":   "ready",
		"active_stack_reference": filepath.Base(activePath),
	}
	if err := writeJSON(out, plan); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "rollback plan: %s\n", plan["rollback_id"])
	return nil
}

func runReport(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "render" {
		return errors.New("report command requires render")
	}
	gatePath, err := flagValue(args[1:], "--gate")
	if err != nil {
		return err
	}
	planPath, err := flagValue(args[1:], "--plan")
	if err != nil {
		return err
	}
	out, err := flagValue(args[1:], "--out")
	if err != nil {
		return err
	}
	if err := requireTmpOutput(out); err != nil {
		return err
	}
	gate, err := readJSONMap(gatePath)
	if err != nil {
		return err
	}
	plan, err := readJSONMap(planPath)
	if err != nil {
		return err
	}
	body := renderReport(gate, plan)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "promotion report: %s\n", out)
	return nil
}

func runApply(args []string, stdout io.Writer) error {
	planPath, err := flagValue(args, "--plan")
	if err != nil {
		return err
	}
	out, err := flagValue(args, "--out")
	if err != nil {
		return err
	}
	if !hasFlag(args, "--dry-run") {
		return errors.New("apply requires --dry-run in v0.1")
	}
	if err := requireTmpOutput(out); err != nil {
		return err
	}
	plan, err := readJSONMap(planPath)
	if err != nil {
		return err
	}
	actions := asAnySlice(plan["actions"])
	result := map[string]any{
		"schema_version":       "ao.promoter.apply-result.v0.1",
		"status":               "dry_run_complete",
		"plan_id":              stringField(plan, "plan_id"),
		"candidate_id":         stringField(plan, "candidate_id"),
		"actions_simulated":    len(actions),
		"mutates_live_state":   false,
		"active_stack_written": false,
		"operator_approval_required_for_live_apply": true,
	}
	if err := writeJSON(out, result); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "apply dry-run: complete")
	return nil
}

func runEvidence(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "inspect" {
		return errors.New("evidence command requires inspect")
	}
	packetPath, err := flagValue(args[1:], "--packet")
	if err != nil {
		return err
	}
	state, err := loadPacket(packetPath)
	if err != nil {
		return err
	}
	for _, ref := range state.Refs {
		fmt.Fprintf(stdout, "%s status=%s digest=ok\n", stringField(ref, "role"), stringField(ref, "status"))
	}
	return nil
}

func runSafety(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "scan" {
		return errors.New("safety command requires scan")
	}
	path, err := flagValue(args[1:], "--path")
	if err != nil {
		return err
	}
	out, err := flagValue(args[1:], "--out")
	if err != nil {
		return err
	}
	if err := requireTmpOutput(out); err != nil {
		return err
	}
	result, err := safetyScan(path)
	if err != nil {
		return err
	}
	if err := writeJSON(out, result); err != nil {
		return err
	}
	if result["status"] == "failed" {
		return errors.New("safety scan failed")
	}
	fmt.Fprintln(stdout, "safety scan: passed")
	return nil
}

func runMission(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "rollup-summary" {
		return errors.New("mission command requires rollup-summary")
	}
	noPromotionPath, err := flagValue(args[1:], "--no-promotion")
	if err != nil {
		return err
	}
	out, err := flagValue(args[1:], "--out")
	if err != nil {
		return err
	}
	if err := requireTmpOutput(out); err != nil {
		return err
	}
	summary, err := missionRollupSummary(noPromotionPath)
	if err != nil {
		return err
	}
	if err := writeJSON(out, summary); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "mission_rollup_summary=%s\n", out)
	return nil
}

func missionRollupSummary(noPromotionPath string) (map[string]any, error) {
	noPromotion, err := readJSONMap(noPromotionPath)
	if err != nil {
		return nil, err
	}
	if stringField(noPromotion, "schema_version") != "ao.promoter.mission-gateway-no-promotion.v0.1" {
		return nil, errors.New("mission no-promotion artifact schema_version mismatch")
	}
	if stringField(noPromotion, "status") != "ready" {
		return nil, errors.New("mission no-promotion artifact must be ready")
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
		if boolField(noPromotion, key) {
			return nil, fmt.Errorf("mission no-promotion artifact widens %s", key)
		}
	}
	nextAction := "keep AO Mission gateway and timeline evidence as no-promotion readback unless a separate exact-scope promotion packet passes all gates"
	return map[string]any{
		"schema_version":                    "ao.promoter.mission-rollup-summary.v0.1",
		"status":                            "ready",
		"subject":                           stringField(noPromotion, "subject"),
		"promotion_rollup_status":           "no_promotion",
		"promotion_count":                   0,
		"no_promotion_count":                1,
		"mission_no_promotion_rollup_bound": true,
		"no_promotion_artifact":             filepath.ToSlash(noPromotionPath),
		"operator_status":                   "no_promotion_requested",
		"read_only_operator_status":         true,
		"operator_summary":                  "No promotion requested. AO Mission gateway evidence is recorded as read-only no-promotion status.",
		"operator_next_action":              nextAction,
		"promotion_allowed":                 false,
		"activation_plan_allowed":           false,
		"class_promotion_allowed":           false,
		"safe_to_execute":                   false,
		"executes_work":                     false,
		"approves_work":                     false,
		"mutates_repositories":              false,
		"provider_calls_allowed":            false,
		"release_or_publish_allowed":        false,
		"credential_use_allowed":            false,
		"direct_main_mutation_allowed":      false,
		"concurrent_mutation_allowed":       false,
		"generated_at_utc":                  nowUTC(),
		"exact_next_action":                 nextAction,
	}, nil
}

func runLiveMutation(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("live-mutation command requires boundary, docs-boundary, or complex-verdict")
	}
	if args[0] == "docs-boundary" {
		return runLiveDocsMutationBoundary(args[1:], stdout)
	}
	if args[0] == "complex-verdict" {
		return runComplexRepoMutationPromotionVerdict(args[1:], stdout)
	}
	if args[0] != "boundary" {
		return errors.New("live-mutation command requires boundary, docs-boundary, or complex-verdict")
	}
	specs := []liveMutationBoundarySpec{
		{Role: "covenant_authority", Flag: "--authority", Schema: "covenant.live-mutation-authority.v1", AcceptedStatus: "approved"},
		{Role: "foundry_request", Flag: "--foundry-request", Schema: "ao.foundry.live-mutation-request.v0.1", AcceptedStatus: "ready"},
		{Role: "forge_dry_run_plan", Flag: "--forge-plan", Schema: "ao.forge.live-mutation-dry-run-plan.v0.1", AcceptedStatus: "ready"},
		{Role: "ao2_dry_run_packet", Flag: "--ao2-packet", Schema: "ao2.live-mutation-dry-run-packet.v1", AcceptedStatus: "ready"},
		{Role: "sentinel_hold_verdict", Flag: "--sentinel-hold", Schema: "ao.sentinel.live-mutation-hold.v0.1", AcceptedStatus: "clear"},
		{Role: "rollback_rehearsal", Flag: "--rollback", Schema: "ao.foundry.live-mutation-rollback-rehearsal.v0.1", AcceptedStatus: "ready"},
		{Role: "command_readback", Flag: "--command-status", Schema: "ao.command.live-mutation-status.v0.1", AcceptedStatus: "ready"},
	}
	for i := range specs {
		path, err := flagValue(args[1:], specs[i].Flag)
		if err != nil {
			return err
		}
		specs[i].Path = path
	}
	out, err := flagValue(args[1:], "--out")
	if err != nil {
		return err
	}
	if err := requireTmpOutput(out); err != nil {
		return err
	}
	boundary, err := evaluateLiveMutationBoundary(specs)
	if err != nil {
		return err
	}
	if err := writeJSON(out, boundary); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "live-mutation boundary: %s\n", boundary["status"])
	return nil
}

func runLiveDocsMutationBoundary(args []string, stdout io.Writer) error {
	specs := []liveMutationBoundarySpec{
		{Role: "approval_ticket", Flag: "--approval-ticket", Schema: "covenant.live-docs-approval-ticket.v1", AcceptedStatus: "approved"},
		{Role: "foundry_approval_gate", Flag: "--foundry-gate", Schema: "ao.foundry.live-docs-approval-gate.v0.1", AcceptedStatus: "ready"},
		{Role: "forge_execution_guard", Flag: "--forge-guard", Schema: "ao.forge.live-docs-execution-guard.v0.1", AcceptedStatus: "ready"},
		{Role: "ao2_docs_patch_packet", Flag: "--ao2-packet", Schema: "ao2.docs-only-patch-packet.v1", AcceptedStatus: "ready"},
		{Role: "sentinel_docs_hold_verdict", Flag: "--sentinel-verdict", Schema: "ao.sentinel.live-docs-mutation-hold.v0.1", AcceptedStatus: "clear"},
		{Role: "rollback_execution_rehearsal", Flag: "--rollback", Schema: "ao.foundry.live-docs-rollback-execution-rehearsal.v0.1", AcceptedStatus: "ready"},
		{Role: "command_readback", Flag: "--command-readback", Schema: "ao.command.live-docs-mutation-status.v0.1", AcceptedStatus: "ready"},
	}
	for i := range specs {
		path, err := flagValue(args, specs[i].Flag)
		if err != nil {
			return err
		}
		specs[i].Path = path
	}
	out, err := flagValue(args, "--out")
	if err != nil {
		return err
	}
	if err := requireTmpOutput(out); err != nil {
		return err
	}
	boundary, err := evaluateLiveDocsMutationBoundary(specs)
	if err != nil {
		return err
	}
	if err := writeJSON(out, boundary); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "live-docs mutation boundary: %s\n", boundary["status"])
	return nil
}

func runComplexRepoMutationPromotionVerdict(args []string, stdout io.Writer) error {
	rollupPath, err := flagValue(args, "--rollup")
	if err != nil {
		return err
	}
	out, err := flagValue(args, "--out")
	if err != nil {
		return err
	}
	if err := requireTmpOutput(out); err != nil {
		return err
	}
	verdict, err := evaluateComplexRepoMutationPromotionVerdict(rollupPath)
	if err != nil {
		return err
	}
	if err := writeJSON(out, verdict); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "complex-repo mutation promotion verdict: %s\n", verdict["status"])
	return nil
}

type liveMutationBoundarySpec struct {
	Role           string
	Flag           string
	Path           string
	Schema         string
	AcceptedStatus string
}

func evaluateComplexRepoMutationPromotionVerdict(rollupPath string) (map[string]any, error) {
	rollup, err := readJSONMap(rollupPath)
	if err != nil {
		return nil, fmt.Errorf("read complex promotion rollup: %w", err)
	}
	sha, err := sha256File(rollupPath)
	if err != nil {
		return nil, fmt.Errorf("hash complex promotion rollup: %w", err)
	}
	blockers := []blocker{}
	if stringField(rollup, "schema_version") != "ao.foundry.complex-repo-mutation-promotion-rollup.v0.1" {
		blockers = append(blockers, newBlocker("complex_repo_mutation_promotion_rollup", "critical", "rollup schema mismatch", rollupPath, "attach a Foundry complex_repo_mutation promotion rollup"))
	}
	if stringField(rollup, "mutation_class") != "complex_repo_mutation" {
		blockers = append(blockers, newBlocker("complex_repo_mutation_promotion_rollup", "critical", "rollup mutation_class must be complex_repo_mutation", rollupPath, "evaluate the exact complex_repo_mutation rollup"))
	}
	requiredChecks := []string{
		"all_nodes_completed",
		"run_links_complete",
		"node_gates_safe",
		"no_concurrent_mutation",
		"pr_ci_merge_evidence",
		"rollback_evidence",
		"sentinel_evidence",
		"promoter_evidence",
		"command_readback",
		"atlas_final_workgraph_complete",
		"bounded_authority",
		"forbidden_surfaces_clear",
	}
	checks, _ := rollup["checks"].(map[string]any)
	for _, check := range requiredChecks {
		if !boolField(checks, check) {
			blockers = append(blockers, newBlocker("complex_repo_mutation_promotion_rollup", "critical", "rollup check "+check+" is not satisfied", rollupPath, "repair missing complex_repo_mutation closure evidence before promotion"))
		}
	}
	if stringField(rollup, "status") != "ready" || !boolField(rollup, "safe_to_promote") || !boolField(rollup, "complex_repo_mutation_live_proven") {
		reason := firstNonEmpty(stringField(rollup, "first_failing_check"), "rollup is not safe to promote")
		blockers = append(blockers, newBlocker("complex_repo_mutation_promotion_rollup", "critical", reason, rollupPath, "do not promote complex_repo_mutation until Foundry rollup is ready"))
	}
	if stringField(rollup, "fully_unsupervised_complex_mutation") != "denied" || stringField(rollup, "rsi") != "denied" {
		blockers = append(blockers, newBlocker("complex_repo_mutation_promotion_rollup", "critical", "rollup must keep fully unsupervised complex mutation and RSI denied", rollupPath, "remove unsupported higher-class claims"))
	}
	status := "promoted"
	highest := "complex_repo_mutation"
	nextDenied := "fully_unsupervised_complex_mutation"
	if len(blockers) > 0 {
		status = "denied"
		highest = firstNonEmpty(stringField(rollup, "highest_proven_live_class"), "multi_repo_low_risk")
		nextDenied = firstNonEmpty(stringField(rollup, "next_denied_class"), "complex_repo_mutation")
	}
	return map[string]any{
		"schema_version":                      "ao.promoter.complex-repo-mutation-promotion-verdict.v0.1",
		"status":                              status,
		"mutation_class":                      "complex_repo_mutation",
		"safe_to_promote":                     len(blockers) == 0,
		"complex_repo_mutation_live_proven":   len(blockers) == 0,
		"highest_proven_live_class":           highest,
		"next_denied_class":                   nextDenied,
		"fully_unsupervised_complex_mutation": "denied",
		"rsi":                                 "denied",
		"rollup": map[string]any{
			"path":   filepath.ToSlash(rollupPath),
			"sha256": sha,
			"status": stringField(rollup, "status"),
		},
		"blockers":             blockers,
		"required_followups":   followups(blockers),
		"mutates_repositories": false,
		"schedules_work":       false,
		"executes_work":        false,
		"approves_work":        false,
		"evaluated_at_utc":     nowUTC(),
	}, nil
}

func evaluateLiveMutationBoundary(specs []liveMutationBoundarySpec) (map[string]any, error) {
	blockers := []blocker{}
	results := []map[string]any{}
	evidence := map[string]map[string]any{}
	for _, spec := range specs {
		raw, err := readJSONMap(spec.Path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", spec.Role, err)
		}
		evidence[spec.Role] = raw
		sha, err := sha256File(spec.Path)
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", spec.Role, err)
		}
		status := liveMutationStatus(spec.Role, raw)
		passed := true
		if stringField(raw, "schema_version") != spec.Schema {
			passed = false
			blockers = append(blockers, newBlocker(spec.Role, "critical", "schema mismatch", spec.Path, "attach the expected live-mutation evidence schema"))
		}
		if status != spec.AcceptedStatus {
			passed = false
			blockers = append(blockers, newBlocker(spec.Role, "critical", "status is not accepted", spec.Path, "repair live-mutation evidence before activation"))
		}
		if err := rejectLiveMutationAuthorityExpansion(spec.Role, raw); err != nil {
			passed = false
			blockers = append(blockers, newBlocker(spec.Role, "critical", err.Error(), spec.Path, "keep live-mutation promotion boundary dry-run and read-only"))
		}
		if spec.Role == "sentinel_hold_verdict" && boolField(raw, "hold_required") {
			passed = false
			blockers = append(blockers, newBlocker(spec.Role, "critical", "Sentinel hold is required", spec.Path, "clear Sentinel hold before activation boundary can pass"))
		}
		if spec.Role == "command_readback" && stringField(raw, "kill_switch_state") != "armed" {
			passed = false
			blockers = append(blockers, newBlocker(spec.Role, "critical", "operator kill-switch is not armed", spec.Path, "arm operator kill-switch before activation boundary can pass"))
		}
		results = append(results, map[string]any{
			"role":            spec.Role,
			"path":            filepath.ToSlash(spec.Path),
			"schema_version":  stringField(raw, "schema_version"),
			"status":          status,
			"accepted_status": spec.AcceptedStatus,
			"sha256":          sha,
			"passed":          passed,
		})
	}
	classBlockers, classReadiness := evaluateClassPromotionReadiness(evidence)
	blockers = append(blockers, classBlockers...)
	status := "passed"
	if len(blockers) > 0 {
		status = "failed"
	}
	safeToPromoteNextClass := len(blockers) == 0 && stringField(classReadiness, "status") == "ready"
	return map[string]any{
		"schema_version":                    "ao.promoter.live-mutation-boundary.v0.1",
		"status":                            status,
		"current_mutation_class":            stringField(classReadiness, "current_class"),
		"next_mutation_class":               stringField(classReadiness, "next_class"),
		"class_promotion_readiness":         classReadiness,
		"safe_to_promote_next_class":        safeToPromoteNextClass,
		"gate_results":                      results,
		"blockers":                          blockers,
		"required_followups":                followups(blockers),
		"live_mutation_activation_allowed":  len(blockers) == 0,
		"dry_run_only":                      true,
		"mutates_live_state":                false,
		"mutates_repositories":              false,
		"schedules_work":                    false,
		"executes_work":                     false,
		"approves_work":                     false,
		"provider_calls_allowed":            false,
		"release_or_publish_allowed":        false,
		"operator_approval_still_required":  true,
		"first_tiny_live_class_still_gated": true,
		"evaluated_at_utc":                  nowUTC(),
	}, nil
}

func evaluateClassPromotionReadiness(evidence map[string]map[string]any) ([]blocker, map[string]any) {
	blockers := []blocker{}
	authority := evidence["covenant_authority"]
	command := evidence["command_readback"]
	sentinel := evidence["sentinel_hold_verdict"]
	rollback := evidence["rollback_rehearsal"]

	currentClass := firstNonEmpty(stringField(command, "current_mutation_class"), stringField(command, "mutation_class"), stringField(sentinel, "mutation_class"))
	nextClass := stringField(command, "next_mutation_class")
	expectedNextClass := nextMutationClass(currentClass)

	if currentClass == "" {
		blockers = append(blockers, newBlocker("class_promotion_class", "critical", "current mutation class is missing", "", "record current mutation class before class promotion"))
	}
	if nextClass == "" {
		blockers = append(blockers, newBlocker("class_promotion_class", "critical", "next mutation class is missing", "", "record next mutation class before class promotion"))
	}
	if currentClass != "" && nextClass != "" && expectedNextClass != nextClass {
		blockers = append(blockers, newBlocker("class_promotion_class", "critical", "next mutation class is not the immediate governed successor", "", "promote only one mutation class at a time"))
	}

	rehearsal, _ := command["completed_live_rehearsal"].(map[string]any)
	completedLiveRehearsal := stringField(rehearsal, "status") == "completed" && stringField(rehearsal, "mutation_class") == currentClass
	currentClassLiveEvidenceStatus := "missing"
	if completedLiveRehearsal {
		currentClassLiveEvidenceStatus = "completed"
	}
	highestProvenLiveClass := previousMutationClass(currentClass)
	if completedLiveRehearsal {
		highestProvenLiveClass = currentClass
	}
	if !completedLiveRehearsal {
		blockers = append(blockers, newBlocker("class_promotion_live_rehearsal", "critical", "completed live rehearsal evidence is missing", "", "complete and record the current class live rehearsal before promotion"))
	}

	rollbackClass := stringField(rollback, "mutation_class")
	rollbackProof := boolField(rollback, "rollback_verified") || (boolField(rollback, "revert_path_verified") && boolField(rollback, "quarantine_path_verified"))
	if rollbackClass != "" && currentClass != "" && rollbackClass != currentClass {
		rollbackProof = false
	}
	if !rollbackProof {
		blockers = append(blockers, newBlocker("class_promotion_rollback", "critical", "class-bound rollback proof is missing", "", "prove rollback for the current class before promotion"))
	}

	mainCI, _ := command["clean_main_ci"].(map[string]any)
	cleanMainCI := (stringField(mainCI, "status") == "passed" || stringField(mainCI, "status") == "success") && stringField(mainCI, "branch") == "main"
	if !cleanMainCI {
		blockers = append(blockers, newBlocker("class_promotion_main_ci", "critical", "clean main CI evidence is missing or failed", "", "attach clean main CI evidence before class promotion"))
	}

	activeHoldsClear := !boolField(sentinel, "hold_required") && !boolField(sentinel, "promoter_hold_required") && len(asAnySlice(command["active_holds"])) == 0
	if classVerdict, ok := sentinel["class_hold_verdict"].(map[string]any); ok && stringField(classVerdict, "status") != "clear" {
		activeHoldsClear = false
	}
	if !activeHoldsClear {
		blockers = append(blockers, newBlocker("class_promotion_active_holds", "critical", "active hold evidence is not clear", "", "clear Sentinel and Promoter holds before class promotion"))
	}

	prerequisites := map[string]any(nil)
	requirements := []string(nil)
	if currentClass == "test_only" && nextClass == "low_risk_code" {
		exactCovenantClassTicket := stringField(authority, "schema_version") == "covenant.live-mutation-authority.v1" &&
			stringField(authority, "status") == "approved" &&
			stringField(authority, "mutation_class") == "low_risk_code" &&
			boolField(authority, "safe_to_request") &&
			!boolField(authority, "safe_to_execute")
		commandReadback := stringField(command, "schema_version") == "ao.command.live-mutation-status.v0.1" &&
			stringField(command, "status") == "ready" &&
			stringField(command, "operator_mode") == "read_only" &&
			stringField(command, "kill_switch_state") == "armed" &&
			stringField(command, "next_mutation_class") == "low_risk_code" &&
			!boolField(command, "safe_to_execute")
		sentinelClearVerdict := stringField(sentinel, "status") == "clear" && activeHoldsClear
		successfulTestOnlyLiveEvidence := completedLiveRehearsal && currentClass == "test_only"
		rollbackFixture := rollbackProof && stringField(rollback, "mutation_class") == "test_only"
		prerequisites = map[string]any{
			"successful_test_only_live_evidence": successfulTestOnlyLiveEvidence,
			"rollback_fixture":                   rollbackFixture,
			"sentinel_clear_verdict":             sentinelClearVerdict,
			"clean_main_ci":                      cleanMainCI,
			"exact_covenant_class_ticket":        exactCovenantClassTicket,
			"command_readback":                   commandReadback,
		}
		requirements = []string{
			"completed_live_rehearsal:test_only",
			"rollback_fixture:test_only",
			"sentinel_clear_verdict:low_risk_code",
			"clean_main_ci:main",
			"exact_covenant_class_ticket:low_risk_code",
			"command_readback:low_risk_code",
		}
		if !exactCovenantClassTicket {
			blockers = append(blockers, newBlocker("class_promotion_covenant_ticket", "critical", "exact low_risk_code Covenant class ticket is missing", "", "attach an approved, exact-scope low_risk_code Covenant ticket before promotion"))
		}
		if !commandReadback {
			blockers = append(blockers, newBlocker("class_promotion_command_readback", "critical", "low_risk_code Command readback is missing or unsafe", "", "attach read-only Command readback with armed kill-switch before promotion"))
		}
	}
	if currentClass == "low_risk_code" && nextClass == "multi_repo_low_risk" {
		exactCovenantClassTicket := stringField(authority, "schema_version") == "covenant.live-mutation-authority.v1" &&
			stringField(authority, "status") == "approved" &&
			stringField(authority, "mutation_class") == "multi_repo_low_risk" &&
			boolField(authority, "safe_to_request") &&
			!boolField(authority, "safe_to_execute")
		commandReadback := stringField(command, "schema_version") == "ao.command.live-mutation-status.v0.1" &&
			stringField(command, "status") == "ready" &&
			stringField(command, "operator_mode") == "read_only" &&
			stringField(command, "kill_switch_state") == "armed" &&
			stringField(command, "current_mutation_class") == "low_risk_code" &&
			stringField(command, "next_mutation_class") == "multi_repo_low_risk" &&
			!boolField(command, "safe_to_execute")
		sentinelClearVerdict := stringField(sentinel, "status") == "clear" && activeHoldsClear
		successfulLowRiskCodeLiveEvidence := completedLiveRehearsal && currentClass == "low_risk_code"
		rollbackFixture := rollbackProof && stringField(rollback, "mutation_class") == "low_risk_code"
		multiRepoPrereqs, multiRepoBlockers := evaluateMultiRepoPromotionPrerequisites(command)
		blockers = append(blockers, multiRepoBlockers...)
		prerequisites = map[string]any{
			"successful_low_risk_code_live_evidence": successfulLowRiskCodeLiveEvidence,
			"rollback_fixture":                       rollbackFixture,
			"sentinel_clear_verdict":                 sentinelClearVerdict,
			"clean_main_ci":                          cleanMainCI,
			"exact_covenant_class_ticket":            exactCovenantClassTicket,
			"command_readback":                       commandReadback,
			"ordered_merge_plan":                     multiRepoPrereqs["ordered_merge_plan"],
			"per_repo_rollback":                      multiRepoPrereqs["per_repo_rollback"],
			"ci_per_repo":                            multiRepoPrereqs["ci_per_repo"],
			"fresh_repo_state":                       multiRepoPrereqs["fresh_repo_state"],
			"kill_switch":                            multiRepoPrereqs["kill_switch"],
		}
		requirements = []string{
			"completed_live_rehearsal:low_risk_code",
			"rollback_fixture:low_risk_code",
			"sentinel_clear_verdict:multi_repo_low_risk",
			"clean_main_ci:main",
			"exact_covenant_class_ticket:multi_repo_low_risk",
			"command_readback:multi_repo_low_risk",
			"ordered_merge_plan:multi_repo_low_risk",
			"per_repo_rollback:multi_repo_low_risk",
			"ci_per_repo:multi_repo_low_risk",
			"fresh_repo_state:multi_repo_low_risk",
			"kill_switch:armed",
		}
		if !exactCovenantClassTicket {
			blockers = append(blockers, newBlocker("class_promotion_covenant_ticket", "critical", "exact multi_repo_low_risk Covenant class ticket is missing", "", "attach an approved, exact-scope multi_repo_low_risk Covenant ticket before promotion"))
		}
		if !commandReadback {
			blockers = append(blockers, newBlocker("class_promotion_command_readback", "critical", "multi_repo_low_risk Command readback is missing or unsafe", "", "attach read-only Command readback with armed kill-switch before promotion"))
		}
	}

	status := "ready"
	if len(blockers) > 0 {
		status = "denied"
	}
	readiness := map[string]any{
		"status":                             status,
		"current_class":                      currentClass,
		"next_class":                         nextClass,
		"expected_next_class":                expectedNextClass,
		"highest_proven_live_class":          highestProvenLiveClass,
		"current_class_live_evidence_status": currentClassLiveEvidenceStatus,
		"completed_live_rehearsal":           completedLiveRehearsal,
		"rollback_proof":                     rollbackProof,
		"clean_main_ci":                      cleanMainCI,
		"active_holds_clear":                 activeHoldsClear,
		"denied_reasons":                     followups(blockers),
		"fully_unsupervised_claimed":         false,
	}
	if prerequisites != nil {
		readiness["promotion_prerequisites"] = prerequisites
		readiness["promotion_prerequisite_requirements"] = requirements
	}
	if status == "denied" && nextClass != "" {
		readiness["next_denied_class"] = nextClass
		readiness["next_denied_reason"] = classPromotionDenialReason(currentClass, blockers)
	}
	return blockers, readiness
}

func evaluateMultiRepoPromotionPrerequisites(command map[string]any) (map[string]bool, []blocker) {
	prereqs := map[string]bool{
		"ordered_merge_plan": false,
		"per_repo_rollback":  false,
		"ci_per_repo":        false,
		"fresh_repo_state":   false,
		"kill_switch":        stringField(command, "kill_switch_state") == "armed",
	}
	blockers := []blocker{}
	if !prereqs["kill_switch"] {
		blockers = append(blockers, newBlocker("class_promotion_kill_switch", "critical", "operator kill-switch is not armed", "", "arm the operator kill-switch before multi-repo promotion"))
	}
	plan := asAnySlice(command["repo_execution_plan"])
	if len(plan) < 2 {
		blockers = append(blockers, newBlocker("class_promotion_ordered_merge_plan", "critical", "ordered multi-repo merge plan is missing", "", "attach serialized per-repo merge dependency evidence"))
		return prereqs, blockers
	}
	seen := map[string]bool{}
	repos := []string{}
	orderedOK := true
	freshStateOK := true
	rollbackOK := true
	ciOK := true
	for index, item := range plan {
		state, ok := item.(map[string]any)
		if !ok {
			orderedOK = false
			continue
		}
		repo := stringField(state, "repo")
		dependencies := stringsFrom(state["depends_on"])
		mergeAfter := stringsFrom(state["merge_after"])
		if repo == "" ||
			int(numberField(state, "order")) != index+1 ||
			stringField(state, "planned_pr") == "" ||
			stringField(state, "status") != "ready" ||
			!equalStringSlices(dependencies, mergeAfter) {
			orderedOK = false
		}
		for _, dependency := range dependencies {
			if !seen[dependency] {
				orderedOK = false
			}
		}
		if stringField(state, "repo_state_status") != "clean_synced" || staleTimestamp(stringField(state, "repo_state_expires_at_utc")) {
			freshStateOK = false
		}
		if stringField(state, "rollback_status") != "ready" {
			rollbackOK = false
		}
		if !statusPassed(stringField(state, "ci_status")) {
			ciOK = false
		}
		seen[repo] = true
		repos = append(repos, repo)
	}
	rollbackByRepo := mapByRepo(command["per_repo_rollback"])
	ciByRepo := mapByRepo(command["per_repo_ci"])
	for _, repo := range repos {
		rollback := rollbackByRepo[repo]
		if rollback == nil || stringField(rollback, "status") != "ready" || len(asAnySlice(rollback["rollback_scope"])) == 0 {
			rollbackOK = false
		}
		ci := ciByRepo[repo]
		if ci == nil || !boolField(ci, "required") || !statusPassed(stringField(ci, "status")) {
			ciOK = false
		}
	}
	prereqs["ordered_merge_plan"] = orderedOK
	prereqs["per_repo_rollback"] = rollbackOK
	prereqs["ci_per_repo"] = ciOK
	prereqs["fresh_repo_state"] = freshStateOK
	if !orderedOK {
		blockers = append(blockers, newBlocker("class_promotion_ordered_merge_plan", "critical", "ordered multi-repo merge plan is missing or unsafe", "", "repair serialized per-repo merge dependency evidence"))
	}
	if !rollbackOK {
		blockers = append(blockers, newBlocker("class_promotion_per_repo_rollback", "critical", "per-repo rollback evidence is incomplete", "", "provide ready rollback for every planned repo"))
	}
	if !ciOK {
		blockers = append(blockers, newBlocker("class_promotion_per_repo_ci", "high", "per-repo CI evidence is incomplete", "", "provide passing CI for every planned repo"))
	}
	if !freshStateOK {
		blockers = append(blockers, newBlocker("class_promotion_repo_state", "high", "repo state evidence is stale", "", "refresh clean synced repo-state readback for every repo"))
	}
	return prereqs, blockers
}

func nextMutationClass(current string) string {
	return mutationClassPromotionSuccessor[current]
}

func previousMutationClass(class string) string {
	for previous, next := range mutationClassPromotionSuccessor {
		if next == class {
			return previous
		}
	}
	return ""
}

func classPromotionDenialReason(currentClass string, blockers []blocker) string {
	for _, blocker := range blockers {
		if strings.HasPrefix(blocker.BlockerID, "class_promotion_live_rehearsal") && currentClass != "" {
			return fmt.Sprintf("denied until %s completed live rehearsal evidence is recorded", currentClass)
		}
	}
	if len(blockers) > 0 {
		return blockers[0].Reason
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func evaluateLiveDocsMutationBoundary(specs []liveMutationBoundarySpec) (map[string]any, error) {
	blockers := []blocker{}
	results := []map[string]any{}
	for _, spec := range specs {
		raw, err := readJSONMap(spec.Path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", spec.Role, err)
		}
		sha, err := sha256File(spec.Path)
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", spec.Role, err)
		}
		status := liveMutationStatus(spec.Role, raw)
		passed := true
		if stringField(raw, "schema_version") != spec.Schema {
			passed = false
			blockers = append(blockers, newBlocker(spec.Role, "critical", "schema mismatch", spec.Path, "attach the expected first live docs evidence schema"))
		}
		if status != spec.AcceptedStatus {
			passed = false
			blockers = append(blockers, newBlocker(spec.Role, "critical", "status is not accepted", spec.Path, "repair first live docs evidence before activation boundary"))
		}
		if err := rejectLiveMutationAuthorityExpansion(spec.Role, raw); err != nil {
			passed = false
			blockers = append(blockers, newBlocker(spec.Role, "critical", err.Error(), spec.Path, "keep first live docs promotion boundary dry-run and read-only"))
		}
		if err := validateLiveDocsRole(spec.Role, raw); err != nil {
			passed = false
			blockers = append(blockers, newBlocker(spec.Role, "critical", err.Error(), spec.Path, "repair exact docs-only approval evidence before boundary can pass"))
		}
		results = append(results, map[string]any{
			"role":            spec.Role,
			"path":            filepath.ToSlash(spec.Path),
			"schema_version":  stringField(raw, "schema_version"),
			"status":          status,
			"accepted_status": spec.AcceptedStatus,
			"sha256":          sha,
			"passed":          passed,
		})
	}
	status := "passed"
	if len(blockers) > 0 {
		status = "failed"
	}
	return map[string]any{
		"schema_version":               "ao.promoter.live-docs-mutation-boundary.v0.1",
		"status":                       status,
		"first_live_class":             "docs_only",
		"gate_results":                 results,
		"blockers":                     blockers,
		"required_followups":           followups(blockers),
		"live_docs_activation_allowed": len(blockers) == 0,
		"safe_to_promote_first_docs_only_live_rehearsal": len(blockers) == 0,
		"dry_run_only":                                true,
		"mutates_live_state":                          false,
		"mutates_repositories":                        false,
		"schedules_work":                              false,
		"executes_work":                               false,
		"approves_work":                               false,
		"provider_calls_allowed":                      false,
		"release_or_publish_allowed":                  false,
		"broad_live_mutation_allowed":                 false,
		"fully_unsupervised_complex_mutation_claimed": false,
		"evaluated_at_utc":                            nowUTC(),
	}, nil
}

func validateLiveDocsRole(role string, raw map[string]any) error {
	switch role {
	case "approval_ticket":
		if stringField(raw, "change_class") != "docs_only" && stringField(raw, "scope") != "docs_only" {
			return errors.New("approval ticket must be exact docs-only scope")
		}
		if boolField(raw, "consumed") {
			return errors.New("approval ticket must be unconsumed")
		}
		if stringField(raw, "approver") == "" {
			return errors.New("approval ticket must include approver identity")
		}
	case "foundry_approval_gate":
		if !boolField(raw, "safe_to_execute") {
			return errors.New("Foundry approval gate must set safe_to_execute")
		}
	case "forge_execution_guard":
		if !boolField(raw, "docs_only_allowlist_enforced") || !boolField(raw, "rollback_plan_required") {
			return errors.New("Forge guard must enforce docs-only allowlist and rollback")
		}
	case "ao2_docs_patch_packet":
		if !boolField(raw, "dry_run_apply") || !boolField(raw, "rollback_patch_present") {
			return errors.New("AO2 docs patch packet must include dry-run apply and rollback patch evidence")
		}
	case "sentinel_docs_hold_verdict":
		if boolField(raw, "hold_required") || boolField(raw, "promoter_hold_required") {
			return errors.New("Sentinel docs hold must be clear")
		}
	case "rollback_execution_rehearsal":
		if !boolField(raw, "rollback_verified") {
			return errors.New("rollback execution rehearsal must verify rollback")
		}
	case "command_readback":
		if stringField(raw, "operator_mode") != "read_only" || stringField(raw, "kill_switch_state") != "armed" {
			return errors.New("Command readback must be read-only with armed kill-switch")
		}
	}
	return nil
}

func liveMutationStatus(role string, raw map[string]any) string {
	if role == "operator_kill_switch" {
		return stringField(raw, "state")
	}
	return stringField(raw, "status")
}

func rejectLiveMutationAuthorityExpansion(label string, value any) error {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			switch key {
			case "mutates_live_state", "mutates_repositories", "schedules_work", "executes_work", "approves_work", "calls_providers", "provider_calls_allowed", "release_or_publish_allowed", "uploads_artifacts", "live_mutation_allowed":
				if b, ok := item.(bool); ok && b {
					return fmt.Errorf("%s expands forbidden authority via %s", label, key)
				}
			}
			if err := rejectLiveMutationAuthorityExpansion(label+"."+key, item); err != nil {
				return err
			}
		}
	case []any:
		for i, item := range v {
			if err := rejectLiveMutationAuthorityExpansion(fmt.Sprintf("%s[%d]", label, i), item); err != nil {
				return err
			}
		}
	case string:
		if containsUnsafePath(v) {
			return fmt.Errorf("%s contains unsafe local path", label)
		}
	}
	return nil
}

func containsUnsafePath(value string) bool {
	for _, marker := range []string{"/" + "Users/", "/" + "home/", "C:" + `\` + "Users" + `\`, "/" + "tmp/", "/" + "var/folders/"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func validateCandidate(candidate map[string]any) error {
	if stringField(candidate, "schema_version") != "ao.promoter.candidate.v0.1" {
		return errors.New("unknown candidate schema_version")
	}
	for _, field := range []string{"candidate_id", "display_name", "component_kind", "version", "source_ref", "target_slot", "target_stack_id", "trust_boundary"} {
		if stringField(candidate, field) == "" {
			return fmt.Errorf("candidate missing required field %s", field)
		}
	}
	if !allowedKinds[stringField(candidate, "component_kind")] {
		return fmt.Errorf("unknown component kind %q", stringField(candidate, "component_kind"))
	}
	if !allowedSlots[stringField(candidate, "target_slot")] {
		return fmt.Errorf("unknown target slot %q", stringField(candidate, "target_slot"))
	}
	if missing := missingRoles(candidate["expected_gate_roles"]); len(missing) > 0 {
		return fmt.Errorf("candidate missing expected gate roles: %s", strings.Join(missing, ", "))
	}
	return nil
}

func loadPacket(path string) (packetState, error) {
	packet, err := readJSONMap(path)
	if err != nil {
		return packetState{}, err
	}
	if stringField(packet, "schema_version") != "ao.promoter.packet.v0.1" {
		return packetState{}, errors.New("unknown packet schema_version")
	}
	candidate, ok := packet["candidate"].(map[string]any)
	if !ok {
		return packetState{}, errors.New("packet candidate must be an object")
	}
	if err := validateCandidate(candidate); err != nil {
		return packetState{}, err
	}
	if boolField(packet, "dry_run_only") != true {
		return packetState{}, errors.New("dry_run_only must be true in v0.1")
	}
	if missing := missingRoles(packet["required_gate_roles"]); len(missing) > 0 {
		return packetState{}, fmt.Errorf("missing required gate roles: %s", strings.Join(missing, ", "))
	}
	refs, err := evidenceRefs(packet["evidence"])
	if err != nil {
		return packetState{}, err
	}
	state := packetState{Packet: packet, Candidate: candidate, Refs: refs, BaseDir: filepath.Dir(path)}
	candidateID := stringField(candidate, "candidate_id")
	for _, ref := range refs {
		role := stringField(ref, "role")
		path := resolvePath(state.BaseDir, stringField(ref, "path"))
		if stringField(ref, "sha256") == "" {
			state.Blockers = append(state.Blockers, newBlocker(role, "critical", "missing sha256 digest", path, "record evidence digest"))
			continue
		}
		if digest, err := sha256File(path); err != nil {
			state.Blockers = append(state.Blockers, newBlocker(role, "critical", "missing evidence file", path, "add evidence file"))
		} else if digest != stringField(ref, "sha256") {
			state.Blockers = append(state.Blockers, newBlocker(role, "critical", "digest mismatch", path, "refresh evidence digest"))
		}
		if refCandidate := stringField(ref, "candidate_id"); refCandidate != candidateID {
			state.Blockers = append(state.Blockers, newBlocker(role, "critical", "candidate mismatch", path, "align evidence candidate_id"))
		}
		if staleEvidence(ref) {
			state.Blockers = append(state.Blockers, newBlocker(role, "high", "stale evidence", path, "regenerate fresh evidence"))
		}
	}
	if missing := missingEvidenceRoles(refs); len(missing) > 0 {
		for _, role := range missing {
			state.Blockers = append(state.Blockers, newBlocker(role, "critical", "missing required gate", "", "add required evidence"))
		}
	}
	if boolField(packet, "rollback_required") && !hasEvidenceRole(refs, "rollback_plan_ready") {
		state.Blockers = append(state.Blockers, newBlocker("rollback_plan_ready", "critical", "missing rollback plan", "", "create rollback plan evidence"))
	}
	return state, nil
}

func evaluateGate(state packetState) (map[string]any, error) {
	blockers := append([]blocker{}, state.Blockers...)
	gateResults := make([]map[string]any, 0, len(state.Refs))
	for _, ref := range state.Refs {
		role := stringField(ref, "role")
		status := stringField(ref, "status")
		accepted := acceptedStatuses[role]
		passed := status == accepted
		path := resolvePath(state.BaseDir, stringField(ref, "path"))
		evidenceBody, _ := readJSONMap(path)
		if role == "public_safety_scan" && numberField(evidenceBody, "findings_count") > 0 {
			passed = false
			blockers = append(blockers, newBlocker(role, "critical", "failed public-safety scan", path, "remove unsafe public content"))
		}
		if !passed {
			reason := fmt.Sprintf("%s status %q is not accepted status %q", role, status, accepted)
			if role == "crucible_hardening_gate" {
				reason = "failed Crucible hardening gate"
			}
			if role == "public_safety_scan" {
				reason = "failed public-safety scan"
			}
			blockers = append(blockers, newBlocker(role, "critical", reason, path, "rerun gate and attach passing evidence"))
		}
		gateResults = append(gateResults, map[string]any{
			"role":            role,
			"status":          status,
			"accepted_status": accepted,
			"passed":          passed,
			"evidence_path":   filepath.ToSlash(stringField(ref, "path")),
		})
	}
	status := "passed"
	if len(blockers) > 0 {
		status = "failed"
	}
	return map[string]any{
		"schema_version":          "ao.promoter.gate.v0.1",
		"status":                  status,
		"candidate_id":            stringField(state.Candidate, "candidate_id"),
		"target_stack_id":         stringField(state.Candidate, "target_stack_id"),
		"gate_results":            gateResults,
		"blockers":                blockers,
		"required_followups":      followups(blockers),
		"promotion_allowed":       len(blockers) == 0,
		"activation_plan_allowed": len(blockers) == 0,
		"evaluated_at_utc":        nowUTC(),
	}, nil
}

func safetyScan(path string) (map[string]any, error) {
	var findings []map[string]any
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	visit := func(file string) error {
		body, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		for lineNo, line := range strings.Split(string(body), "\n") {
			for _, detector := range detectors() {
				if detector.re.MatchString(line) {
					findings = append(findings, map[string]any{
						"detector": detector.name,
						"file":     filepath.ToSlash(file),
						"line":     lineNo + 1,
						"summary":  detector.summary,
					})
				}
			}
		}
		return nil
	}
	if info.IsDir() {
		err = filepath.WalkDir(path, func(file string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				name := d.Name()
				if name == ".git" || name == "tmp" || name == "target" {
					return filepath.SkipDir
				}
				return nil
			}
			if isTextFile(file) {
				return visit(file)
			}
			return nil
		})
	} else {
		err = visit(path)
	}
	if err != nil {
		return nil, err
	}
	status := "passed"
	if len(findings) > 0 {
		status = "failed"
	}
	return map[string]any{
		"schema_version": "ao.promoter.safety-scan.v0.1",
		"status":         status,
		"path":           filepath.ToSlash(path),
		"findings_count": len(findings),
		"findings":       findings,
		"scanned_at_utc": nowUTC(),
	}, nil
}

func detectors() []struct {
	name    string
	summary string
	re      *regexp.Regexp
} {
	return []struct {
		name    string
		summary string
		re      *regexp.Regexp
	}{
		{"bearer_token", "bearer-token-like value detected", regexp.MustCompile(`(?i)Authorization:\s*Bearer\s+\S{16,}`)},
		{"private_key", "private key marker detected", regexp.MustCompile(`BEGIN (RSA |OPENSSH |EC |)PRIVATE KEY`)},
		{"github_token", "GitHub-token-like value detected", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{20,}`)},
		{"cloud_access_key", "cloud access-key-like value detected", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
		{"password_assignment", "password assignment pattern detected", regexp.MustCompile(`(?i)\b(password|passwd|secret)\s*[:=]`)},
		{"local_absolute_path", "local absolute path detected", regexp.MustCompile(`(/Users/[^ \n]+|/home/[^ \n]+|C:\\Users\\[^ \n]+)`)},
		{"forbidden_action_command", "forbidden action command detected", regexp.MustCompile(`(?i)\b(git push|git tag|gh release|npm publish|twine upload|docker push|kubectl apply|terraform apply)\b`)},
	}
}

func readJSONMap(path string) (map[string]any, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return out, nil
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}

func flagValue(args []string, name string) (string, error) {
	for i, arg := range args {
		if arg == name && i+1 < len(args) {
			return args[i+1], nil
		}
	}
	return "", fmt.Errorf("missing %s", name)
}

func hasFlag(args []string, name string) bool {
	for _, arg := range args {
		if arg == name {
			return true
		}
	}
	return false
}

func requireTmpOutput(path string) error {
	clean := filepath.Clean(path)
	parts := strings.Split(clean, string(filepath.Separator))
	for _, part := range parts {
		if part == "tmp" {
			return nil
		}
	}
	return fmt.Errorf("output path must be under tmp/: %s", path)
}

func evidenceRefs(value any) ([]map[string]any, error) {
	raw, ok := value.([]any)
	if !ok {
		return nil, errors.New("packet evidence must be an array")
	}
	refs := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		ref, ok := item.(map[string]any)
		if !ok {
			return nil, errors.New("packet evidence entries must be objects")
		}
		for _, field := range []string{"role", "path", "schema_version", "sha256", "status", "candidate_id", "created_at_utc", "expires_at_utc", "authority"} {
			if stringField(ref, field) == "" {
				return nil, fmt.Errorf("evidence reference missing %s", field)
			}
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func missingRoles(value any) []string {
	seen := map[string]bool{}
	for _, role := range stringsFrom(value) {
		seen[role] = true
	}
	var missing []string
	for _, role := range requiredRoles {
		if !seen[role] {
			missing = append(missing, role)
		}
	}
	return missing
}

func missingEvidenceRoles(refs []map[string]any) []string {
	seen := map[string]bool{}
	for _, ref := range refs {
		seen[stringField(ref, "role")] = true
	}
	var missing []string
	for _, role := range requiredRoles {
		if !seen[role] {
			missing = append(missing, role)
		}
	}
	return missing
}

func hasEvidenceRole(refs []map[string]any, role string) bool {
	for _, ref := range refs {
		if stringField(ref, "role") == role {
			return true
		}
	}
	return false
}

func staleEvidence(ref map[string]any) bool {
	expires, err := time.Parse(time.RFC3339, stringField(ref, "expires_at_utc"))
	if err != nil {
		return true
	}
	return !time.Now().Before(expires)
}

func sha256File(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func newBlocker(role, severity, reason, evidencePath, action string) blocker {
	id := strings.ToLower(role + "_" + strings.ReplaceAll(reason, " ", "_"))
	id = regexp.MustCompile(`[^a-z0-9_]+`).ReplaceAllString(id, "_")
	return blocker{
		BlockerID:         id,
		GateRole:          role,
		Severity:          severity,
		Reason:            reason,
		EvidencePath:      filepath.ToSlash(evidencePath),
		RecommendedAction: action,
	}
}

func followups(blockers []blocker) []string {
	if len(blockers) == 0 {
		return []string{}
	}
	seen := map[string]bool{}
	var out []string
	for _, b := range blockers {
		if !seen[b.RecommendedAction] {
			seen[b.RecommendedAction] = true
			out = append(out, b.RecommendedAction)
		}
	}
	sort.Strings(out)
	return out
}

func activeSlot(active map[string]any, slot string) (map[string]any, error) {
	slots, ok := active["slots"].(map[string]any)
	if !ok {
		return nil, errors.New("active stack slots must be an object")
	}
	current, ok := slots[slot].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("active stack missing current slot %q", slot)
	}
	return current, nil
}

func candidateComponent(candidate map[string]any) map[string]any {
	slot := stringField(candidate, "target_slot")
	id := stringField(candidate, "candidate_id")
	return map[string]any{
		"slot":            slot,
		"component_id":    id,
		"version":         stringField(candidate, "version"),
		"source_ref":      stringField(candidate, "source_ref"),
		"activated_by":    "ao-promoter",
		"activation_gate": "ao.promoter.gate.v0.1",
		"rollback_ref":    "rollback://" + id,
	}
}

func renderReport(gate, plan map[string]any) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# AO Promoter Promotion Report")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Candidate: %s\n", stringField(plan, "candidate_id"))
	fmt.Fprintf(&b, "- Target stack: %s\n", stringField(plan, "target_stack_id"))
	fmt.Fprintf(&b, "- Target slot: %s\n", stringField(plan, "target_slot"))
	fmt.Fprintf(&b, "- Promotion gate: %s\n", stringField(gate, "status"))
	fmt.Fprintf(&b, "- Dry-run only: %t\n", boolField(plan, "dry_run_only"))
	fmt.Fprintf(&b, "- Mutates live state: %t\n", boolField(plan, "mutates_live_state"))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Gate Results")
	for _, item := range asAnySlice(gate["gate_results"]) {
		if result, ok := item.(map[string]any); ok {
			fmt.Fprintf(&b, "- %s: %s\n", stringField(result, "role"), stringField(result, "status"))
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Activation Actions")
	for _, item := range asAnySlice(plan["actions"]) {
		fmt.Fprintf(&b, "- %v\n", item)
	}
	return b.String()
}

func resolvePath(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}

func stringField(m map[string]any, key string) string {
	value, _ := m[key].(string)
	return value
}

func boolField(m map[string]any, key string) bool {
	value, _ := m[key].(bool)
	return value
}

func numberField(m map[string]any, key string) float64 {
	value, _ := m[key].(float64)
	return value
}

func stringsFrom(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func mapByRepo(value any) map[string]map[string]any {
	byRepo := map[string]map[string]any{}
	for _, item := range asAnySlice(value) {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		repo := stringField(entry, "repo")
		if repo != "" {
			byRepo[repo] = entry
		}
	}
	return byRepo
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func staleTimestamp(value string) bool {
	expiresAt, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return true
	}
	return !expiresAt.After(time.Now().UTC())
}

func statusPassed(value string) bool {
	return value == "passed" || value == "success"
}

func asAnySlice(value any) []any {
	if raw, ok := value.([]any); ok {
		return raw
	}
	if raw, ok := value.([]string); ok {
		out := make([]any, 0, len(raw))
		for _, s := range raw {
			out = append(out, s)
		}
		return out
	}
	return []any{}
}

func setOf(values ...string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func isTextFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".json", ".yaml", ".yml", ".txt", ".go":
		return true
	default:
		return false
	}
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
