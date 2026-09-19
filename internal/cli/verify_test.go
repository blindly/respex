package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blindly/respex/internal/state"
)

func TestVerifyNoCommits(t *testing.T) {
	setupProject(t)
	var out, errOut bytes.Buffer
	if code := runVerify(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "no committed versions yet") {
		t.Fatalf("verify without commit: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestVerifyRequiresConfig(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runVerify(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "no [verify] checks configured") {
		t.Fatalf("verify without config: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestVerifyDirtySpecFails(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, fmt.Sprintf("[verify]\ncommands = [[%q]]\n", fakeBin))
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	writeSpec(t, root, "# one\nchanged\n")
	var out, errOut bytes.Buffer
	if code := runVerify(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "spec changed since last commit") {
		t.Fatalf("verify dirty: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestVerifyPassRecordsRow(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, fmt.Sprintf("[verify]\ncommands = [[%q], [%q]]\n", fakeBin, fakeBin))
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runVerify(nil, &out, &errOut); code != 0 || !strings.Contains(out.String(), "verifying v1: passed") {
		t.Fatalf("verify pass: %d, %s | %s", code, out.String(), errOut.String())
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, err := st.LatestVerification(1)
	if err != nil || v == nil || v.Outcome != "passed" || v.LogPath != ".respex/logs/1-verify.log" || v.FinishedAt == nil {
		t.Fatalf("LatestVerification = %+v, %v", v, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".respex", "logs", "1-verify.log")); err != nil {
		t.Fatalf("verify log: %v", err)
	}
}

func TestVerifyStopsAtFirstFailure(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, fmt.Sprintf("[verify]\ncommands = [[%q, \"-fail\"], [%q]]\n", fakeBin, fakeBin))
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runVerify(nil, &out, &errOut); code != 1 ||
		!strings.Contains(out.String(), "[fail]") ||
		!strings.Contains(out.String(), "[skip]") {
		t.Fatalf("verify fail: %d, %s | %s", code, out.String(), errOut.String())
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, err := st.LatestVerification(1)
	if err != nil || v == nil || v.Outcome != "failed" {
		t.Fatalf("LatestVerification = %+v, %v", v, err)
	}
	if !strings.Contains(v.Detail, "skipped") {
		t.Fatalf("detail should record the skipped command: %s", v.Detail)
	}
}

func TestVerifyMissingBinary(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, "[verify]\ncommands = [[\"respex-no-such-binary-xyz\"]]\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runVerify(nil, &out, &errOut); code != 1 || !strings.Contains(out.String(), "respex-no-such-binary-xyz") {
		t.Fatalf("missing binary: %d, %s | %s", code, out.String(), errOut.String())
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, err := st.LatestVerification(1)
	if err != nil || v == nil || v.Outcome != "error" || v.FinishedAt == nil {
		t.Fatalf("LatestVerification = %+v, %v; want finished error row", v, err)
	}
}

func TestVerifyTimeout(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, fmt.Sprintf("[verify]\ncommands = [[%q, \"-sleep\", \"5s\"]]\ntimeout = \"20ms\"\n", fakeBin))
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runVerify(nil, &out, &errOut); code != 1 || !strings.Contains(out.String(), "timed out after 20ms") {
		t.Fatalf("timeout: %d, %s | %s", code, out.String(), errOut.String())
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, err := st.LatestVerification(1)
	if err != nil || v == nil || v.Outcome != "timed_out" {
		t.Fatalf("LatestVerification = %+v, %v", v, err)
	}
}

func TestApplyChainsVerification(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"{{prompt}}\"]\n[verify]\ncommands = [[%q]]\n", fakeBin, fakeBin))
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runApply(nil, &out, &errOut); code != 0 || !strings.Contains(out.String(), "verifying v1: passed") {
		t.Fatalf("apply chains verification: %d, %s | %s", code, out.String(), errOut.String())
	}
	out.Reset()
	if code := runApply(nil, &out, &bytes.Buffer{}); code != 0 || !strings.Contains(out.String(), "nothing to do (v1 already applied)") {
		t.Fatalf("conformed no-op: %d, %s", code, out.String())
	}
}

func TestFailedVerificationTriggersReapply(t *testing.T) {
	root := setupProject(t)
	marker := filepath.Join(root, "marker")
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-marker\", %q, \"{{prompt}}\"]\n[verify]\ncommands = [[%q, \"-fail\"]]\n", fakeBin, marker, fakeBin))
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runApply(nil, &out, &errOut); code != 1 || !strings.Contains(out.String(), "verifying v1: failed") {
		t.Fatalf("apply with failing verify: %d, %s | %s", code, out.String(), errOut.String())
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	// The apply row itself still succeeded.
	applies, err := st.ListApplies()
	if err != nil || len(applies) != 1 || applies[0].ExitCode == nil || *applies[0].ExitCode != 0 {
		t.Fatalf("applies = %+v, %v", applies, err)
	}
	// The failed verification makes the next apply re-run the agent.
	count1, _ := os.ReadFile(marker)
	out.Reset()
	if code := runApply(nil, &out, &bytes.Buffer{}); code != 1 ||
		!strings.Contains(out.String(), "v1 was applied but verification failed") {
		t.Fatalf("re-apply: %d, %s", code, out.String())
	}
	count2, _ := os.ReadFile(marker)
	if len(count2) <= len(count1) {
		t.Fatal("agent should have re-run after a failed verification")
	}
	// Fix the verify command: the re-apply then passes and a further apply
	// is a no-op again.
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-marker\", %q, \"{{prompt}}\"]\n[verify]\ncommands = [[%q]]\n", fakeBin, marker, fakeBin))
	out.Reset()
	if code := runApply(nil, &out, &bytes.Buffer{}); code != 0 || !strings.Contains(out.String(), "verifying v1: passed") {
		t.Fatalf("fixed verify: %d, %s", code, out.String())
	}
	out.Reset()
	if code := runApply(nil, &out, &bytes.Buffer{}); code != 0 || !strings.Contains(out.String(), "nothing to do") {
		t.Fatalf("final no-op: %d, %s", code, out.String())
	}
	verifications, err := st.ListVerifications()
	if err != nil || len(verifications) != 3 {
		t.Fatalf("verifications = %+v, %v", verifications, err)
	}
}

func TestStatusShowsVerifiedLine(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"{{prompt}}\"]\n[verify]\ncommands = [[%q]]\n", fakeBin, fakeBin))
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runApply(nil, &out, &errOut); code != 0 {
		t.Fatalf("apply: %d, %s | %s", code, out.String(), errOut.String())
	}
	var sout bytes.Buffer
	if code := runStatus(nil, &sout, &bytes.Buffer{}); code != 0 || !strings.Contains(sout.String(), "verified:    passed") {
		t.Fatalf("status verified line: %d, %s", code, sout.String())
	}
}

func TestVerifyAuditPass(t *testing.T) {
	root := setupProject(t)
	marker := filepath.Join(t.TempDir(), "audit-marker")
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-conforms\", \"yes\", \"-marker\", %q, \"{{prompt}}\"]\n\n[verify]\ncommands = [[%q]]\naudit = true\n", fakeBin, marker, fakeBin))
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runVerify(nil, &out, &errOut); code != 0 ||
		!strings.Contains(out.String(), "[audit] pass") ||
		!strings.Contains(out.String(), "verifying v1: passed") {
		t.Fatalf("verify audit pass: %d, %s | %s", code, out.String(), errOut.String())
	}
	// The audit agent received the audit prompt pointing at the committed
	// spec snapshot, not the working spec file.
	body, err := os.ReadFile(marker)
	if err != nil || !strings.Contains(string(body), "Audit whether this repository conforms") || !strings.Contains(string(body), ".respex/tmp") {
		t.Fatalf("audit prompt marker = %q, %v", string(body), err)
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, err := st.LatestVerification(1)
	if err != nil || v == nil || v.Outcome != "passed" || v.Agent != fakeBin {
		t.Fatalf("LatestVerification = %+v, %v", v, err)
	}
	var detail verifyDetail
	if err := json.Unmarshal([]byte(v.Detail), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Results) != 1 || detail.Results[0].Status != "passed" {
		t.Fatalf("detail results = %+v", detail.Results)
	}
	if detail.Audit == nil || detail.Audit.Status != "passed" || detail.Audit.Verdict != "yes" || detail.Audit.Agent != fakeBin {
		t.Fatalf("detail audit = %+v", detail.Audit)
	}
}

func TestVerifyAuditOnly(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-conforms\", \"yes\", \"{{prompt}}\"]\n\n[verify]\naudit = true\n", fakeBin))
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runVerify(nil, &out, &errOut); code != 0 ||
		!strings.Contains(out.String(), "[audit] pass") ||
		!strings.Contains(out.String(), "verifying v1: passed") {
		t.Fatalf("audit-only verify: %d, %s | %s", code, out.String(), errOut.String())
	}
	// status reflects the audit-only configuration.
	var sout bytes.Buffer
	if code := runStatus(nil, &sout, &bytes.Buffer{}); code != 0 || !strings.Contains(sout.String(), "verified:    passed") {
		t.Fatalf("status verified line for audit-only: %d, %s", code, sout.String())
	}
}

func TestVerifyAuditNonConforming(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-conforms\", \"no\", \"{{prompt}}\"]\n\n[verify]\ncommands = [[%q]]\naudit = true\n", fakeBin, fakeBin))
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runVerify(nil, &out, &errOut); code != 1 ||
		!strings.Contains(out.String(), "[audit] fail — agent reported non-conformance") ||
		!strings.Contains(out.String(), "verifying v1: failed") {
		t.Fatalf("non-conforming audit: %d, %s | %s", code, out.String(), errOut.String())
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, err := st.LatestVerification(1)
	if err != nil || v == nil || v.Outcome != "failed" {
		t.Fatalf("LatestVerification = %+v, %v", v, err)
	}
	var detail verifyDetail
	if err := json.Unmarshal([]byte(v.Detail), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Audit == nil || detail.Audit.Status != "failed" || detail.Audit.Verdict != "no" {
		t.Fatalf("detail audit = %+v", detail.Audit)
	}
}

func TestVerifyAuditNoVerdict(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"{{prompt}}\"]\n\n[verify]\ncommands = [[%q]]\naudit = true\n", fakeBin, fakeBin))
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runVerify(nil, &out, &errOut); code != 1 ||
		!strings.Contains(out.String(), "no CONFORMS: yes/no verdict in agent output") ||
		!strings.Contains(out.String(), "verifying v1: error") {
		t.Fatalf("audit without verdict: %d, %s | %s", code, out.String(), errOut.String())
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, err := st.LatestVerification(1)
	if err != nil || v == nil || v.Outcome != "error" {
		t.Fatalf("LatestVerification = %+v, %v", v, err)
	}
	var detail verifyDetail
	if err := json.Unmarshal([]byte(v.Detail), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Audit == nil || detail.Audit.Status != "error" || detail.Audit.Verdict != "" {
		t.Fatalf("detail audit = %+v", detail.Audit)
	}
}

func TestVerifyAuditYesNonzeroExit(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-conforms\", \"yes\", \"-fail\", \"{{prompt}}\"]\n\n[verify]\naudit = true\n", fakeBin))
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runVerify(nil, &out, &errOut); code != 1 ||
		!strings.Contains(out.String(), "agent reported conformance but exited 1") ||
		!strings.Contains(out.String(), "verifying v1: error") {
		t.Fatalf("audit yes+nonzero exit: %d, %s | %s", code, out.String(), errOut.String())
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, err := st.LatestVerification(1)
	if err != nil || v == nil || v.Outcome != "error" {
		t.Fatalf("LatestVerification = %+v, %v", v, err)
	}
	var detail verifyDetail
	if err := json.Unmarshal([]byte(v.Detail), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Audit == nil || detail.Audit.Status != "error" || detail.Audit.Verdict != "yes" || detail.Audit.Exit != 1 {
		t.Fatalf("detail audit = %+v", detail.Audit)
	}
}

func TestVerifyAuditTimeout(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-sleep\", \"5s\", \"-conforms\", \"yes\", \"{{prompt}}\"]\n\n[verify]\naudit = true\ntimeout = \"20ms\"\n", fakeBin))
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runVerify(nil, &out, &errOut); code != 1 ||
		!strings.Contains(out.String(), "verifying v1: timed out after 20ms") {
		t.Fatalf("audit timeout: %d, %s | %s", code, out.String(), errOut.String())
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, err := st.LatestVerification(1)
	if err != nil || v == nil || v.Outcome != "timed_out" {
		t.Fatalf("LatestVerification = %+v, %v", v, err)
	}
	var detail verifyDetail
	if err := json.Unmarshal([]byte(v.Detail), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Audit == nil || detail.Audit.Status != "timed_out" {
		t.Fatalf("detail audit = %+v", detail.Audit)
	}
}

func TestVerifyAuditSkippedWhenCommandFails(t *testing.T) {
	root := setupProject(t)
	marker := filepath.Join(t.TempDir(), "audit-marker")
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-conforms\", \"yes\", \"-marker\", %q, \"{{prompt}}\"]\n\n[verify]\ncommands = [[%q, \"-fail\"]]\naudit = true\n", fakeBin, marker, fakeBin))
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runVerify(nil, &out, &errOut); code != 1 ||
		!strings.Contains(out.String(), "[fail]") ||
		!strings.Contains(out.String(), "[audit] skip") ||
		!strings.Contains(out.String(), "verifying v1: failed") {
		t.Fatalf("audit skipped: %d, %s | %s", code, out.String(), errOut.String())
	}
	// The audit agent must not have run.
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("audit agent should not run when commands fail: %v", err)
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, err := st.LatestVerification(1)
	if err != nil || v == nil || v.Outcome != "failed" {
		t.Fatalf("LatestVerification = %+v, %v", v, err)
	}
	var detail verifyDetail
	if err := json.Unmarshal([]byte(v.Detail), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Audit == nil || detail.Audit.Status != "skipped" {
		t.Fatalf("detail audit = %+v", detail.Audit)
	}
}

func TestVerifyAuditRequiresAgent(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, "[verify]\ncommands = [[\"/bin/true\"]]\naudit = true\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runVerify(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "verify audit requires an agent") {
		t.Fatalf("audit without agent: %d, %s | %s", code, out.String(), errOut.String())
	}
}
