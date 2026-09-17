package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/blindly/respex/internal/agent"
	"github.com/blindly/respex/internal/spec"
	"github.com/blindly/respex/internal/state"
	"github.com/blindly/respex/internal/ui"
)

// splitProposal is the JSON payload stored in baselines.proposal for
// `baseline --split` rows: the proposed bundle plus the live content of every
// file it would replace, so review and accept stay self-contained.
type splitProposal struct {
	Dir    string      `json:"dir"`
	Before []spec.File `json:"before"`
	After  []spec.File `json:"after"`
}

var featureNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.md$`)

const maxSplitFeatures = 8

func validFeaturePath(rel string) bool {
	rest, ok := strings.CutPrefix(rel, "specs/")
	if !ok || strings.Contains(rest, "/") {
		return false
	}
	return featureNameRE.MatchString(rest)
}

// collectProposal scans the agent-written proposal directory into an ordered
// bundle: the master spec first, then specs/<feature>.md files sorted by path.
// Anything else is reported as ignored.
func collectProposal(dir, master string) (after []spec.File, ignored []string, err error) {
	var features []spec.File
	var masterFile *spec.File
	walkErr := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		content, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		switch {
		case rel == master:
			f := spec.File{Path: rel, Content: content}
			masterFile = &f
		case validFeaturePath(rel):
			features = append(features, spec.File{Path: rel, Content: content})
		default:
			ignored = append(ignored, rel)
		}
		return nil
	})
	if walkErr != nil {
		return nil, nil, walkErr
	}
	if masterFile == nil {
		return nil, ignored, fmt.Errorf("the agent did not write %s inside the proposal directory", master)
	}
	if len(features) == 0 {
		return nil, ignored, errors.New("the agent did not write any specs/<feature>.md files")
	}
	if len(features) > maxSplitFeatures {
		return nil, ignored, fmt.Errorf("the proposal contains %d feature files (maximum %d)", len(features), maxSplitFeatures)
	}
	sort.Slice(features, func(i, j int) bool { return features[i].Path < features[j].Path })
	after = append([]spec.File{*masterFile}, features...)
	for _, f := range after {
		if len(f.Content) == 0 {
			return nil, ignored, fmt.Errorf("proposed file %s is empty", f.Path)
		}
	}
	return after, ignored, nil
}

// proposalBefore snapshots the live content of every proposal target that
// already exists in the repository.
func proposalBefore(root string, after []spec.File) ([]spec.File, error) {
	var before []spec.File
	for _, f := range after {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		before = append(before, spec.File{Path: f.Path, Content: content})
	}
	return before, nil
}

func pendingProposal(st *state.DB) (*state.Baseline, error) {
	latest, err := st.LatestBaseline()
	if err != nil || latest == nil || latest.Outcome != "proposed" || len(latest.Proposal) == 0 {
		return nil, err
	}
	return latest, nil
}

func decodeProposal(b *state.Baseline) (*splitProposal, error) {
	var p splitProposal
	if err := json.Unmarshal(b.Proposal, &p); err != nil || len(p.After) == 0 {
		return nil, fmt.Errorf("corrupt proposal in baseline #%d", b.ID)
	}
	return &p, nil
}

func removeProposalDir(root, dir string) {
	if strings.HasPrefix(dir, ".respex/proposals/") && !strings.Contains(dir, "..") {
		_ = os.RemoveAll(filepath.Join(root, filepath.FromSlash(dir)))
	}
}

// addSpecFilesConfig inserts a top-level spec_files key into the project
// config, preserving existing content and comments. The key is placed before
// the first table header so it lands in the top-level table.
func addSpecFilesConfig(path string, files []string) error {
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var probe struct {
		SpecFiles []string `toml:"spec_files"`
	}
	if len(raw) > 0 {
		if err := toml.Unmarshal(raw, &probe); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		if len(probe.SpecFiles) > 0 {
			return errors.New("spec_files is already configured")
		}
	}
	quoted := make([]string, len(files))
	for i, f := range files {
		quoted[i] = strconv.Quote(f)
	}
	entry := "spec_files = [" + strings.Join(quoted, ", ") + "]\n"
	text := string(raw)
	insert := len(text)
	offset := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			insert = offset
			break
		}
		offset += len(line)
	}
	if insert < len(text) {
		text = text[:insert] + entry + "\n" + text[insert:]
	} else {
		if len(text) > 0 && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += entry
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func runBaselineAccept(w *workspace, st *state.DB, out, errOut io.Writer) int {
	b, err := pendingProposal(st)
	if err != nil {
		return fail(errOut, err)
	}
	if b == nil {
		return fail(errOut, errors.New("no pending split baseline proposal"))
	}
	p, err := decodeProposal(b)
	if err != nil {
		return fail(errOut, err)
	}
	if len(w.cfg.SpecFiles) > 0 {
		return fail(errOut, errors.New("spec_files is already configured — resolve it manually before accepting"))
	}
	beforeByPath := make(map[string][]byte, len(p.Before))
	for _, f := range p.Before {
		beforeByPath[f.Path] = f.Content
	}
	for _, f := range p.After {
		live, err := os.ReadFile(filepath.Join(w.root, filepath.FromSlash(f.Path)))
		if prev, ok := beforeByPath[f.Path]; ok {
			if err != nil || !bytes.Equal(live, prev) {
				return fail(errOut, fmt.Errorf("spec file %s changed since the proposal was generated — resolve it or `respex baseline discard`", f.Path))
			}
			continue
		}
		if err == nil {
			return fail(errOut, fmt.Errorf("spec file %s already exists — remove it or `respex baseline discard`", f.Path))
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return fail(errOut, err)
		}
	}
	// Install feature files first and the master spec last, so a mid-write
	// failure leaves the old master spec pointing at no new files.
	for _, f := range p.After[1:] {
		if err := writeFileAtomic(filepath.Join(w.root, filepath.FromSlash(f.Path)), f.Content); err != nil {
			return fail(errOut, fmt.Errorf("install %s: %w", f.Path, err))
		}
	}
	if err := writeFileAtomic(filepath.Join(w.root, filepath.FromSlash(p.After[0].Path)), p.After[0].Content); err != nil {
		return fail(errOut, fmt.Errorf("install %s: %w", p.After[0].Path, err))
	}
	featurePaths := make([]string, 0, len(p.After)-1)
	for _, f := range p.After[1:] {
		featurePaths = append(featurePaths, f.Path)
	}
	if err := addSpecFilesConfig(filepath.Join(w.root, ".respex", "config.toml"), featurePaths); err != nil {
		return fail(errOut, fmt.Errorf("spec files installed but config update failed: %w — add spec_files to .respex/config.toml manually", err))
	}
	if err := st.SetBaselineOutcome(b.ID, "accepted"); err != nil {
		return fail(errOut, err)
	}
	removeProposalDir(w.root, p.Dir)
	warnMissingSections(out, p.After[0].Content)
	fmt.Fprintf(out, "accepted split baseline #%d (%d files)\n", b.ID, len(p.After))
	for _, f := range p.After {
		fmt.Fprintf(out, "  %s\n", f.Path)
	}
	fmt.Fprintln(out, "run `respex commit` to snapshot the new spec bundle")
	return 0
}

func runBaselineDiscard(w *workspace, st *state.DB, out, errOut io.Writer) int {
	b, err := pendingProposal(st)
	if err != nil {
		return fail(errOut, err)
	}
	if b == nil {
		return fail(errOut, errors.New("no pending split baseline proposal"))
	}
	if p, err := decodeProposal(b); err == nil {
		removeProposalDir(w.root, p.Dir)
	}
	if err := st.SetBaselineOutcome(b.ID, "discarded"); err != nil {
		return fail(errOut, err)
	}
	fmt.Fprintf(out, "discarded split baseline proposal #%d\n", b.ID)
	return 0
}

func runBaselineSplit(w *workspace, st *state.DB, intent string, merge, noProgress bool, before []byte, out, errOut io.Writer) int {
	if len(w.cfg.SpecFiles) > 0 {
		return fail(errOut, errors.New("spec_files is already configured — remove it from .respex/config.toml to re-propose a split"))
	}
	a, name, err := w.adapter()
	if err != nil {
		return fail(errOut, err)
	}
	master, err := spec.CleanSpecPath(w.cfg.Spec)
	if err != nil {
		return fail(errOut, err)
	}
	proposalsDir := filepath.Join(w.root, ".respex", "proposals")
	if err := os.MkdirAll(proposalsDir, 0o755); err != nil {
		return fail(errOut, err)
	}
	proposalDir, err := os.MkdirTemp(proposalsDir, "baseline-*")
	if err != nil {
		return fail(errOut, err)
	}
	proposalRel, err := filepath.Rel(w.root, proposalDir)
	if err != nil {
		return fail(errOut, err)
	}
	proposalRel = filepath.ToSlash(proposalRel)
	logsDir := filepath.Join(w.root, ".respex", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fail(errOut, err)
	}
	f, err := os.CreateTemp(logsDir, time.Now().UTC().Format("20060102T150405Z")+"-baseline-*.log")
	if err != nil {
		return fail(errOut, err)
	}
	defer f.Close()
	logPath := f.Name()
	logRel := filepath.Join(".respex", "logs", filepath.Base(logPath))
	started := time.Now()
	beforeHash := spec.Hash(before)
	record := func(outcome string, after, proposal []byte, afterHash string) error {
		if after == nil {
			after = []byte{}
		}
		_, err := st.InsertBaseline(name, outcome, beforeHash, afterHash, before, after, proposal, logRel, started, time.Now())
		return err
	}
	tmpl := agent.PromptBaselineSplit
	if w.cfg.Prompts.Baseline != "" {
		tmpl = w.cfg.Prompts.Baseline
	}
	if merge {
		tmpl += "\n\nMerge findings with the existing specification's content. Preserve established user intent and explicitly documented requirements unless they directly contradict observed behavior; record conflicts in Open Questions."
	}
	label := fmt.Sprintf("proposing split baseline via %s", name)
	progressEnabled := ui.IsTTY(out) && !noProgress && os.Getenv("NO_COLOR") == ""
	if !progressEnabled {
		fmt.Fprintf(out, "%s; output: %s\n", label, logRel)
	}
	progress := ui.StartProgress(out, label, progressEnabled)
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, w.cfg.AgentTimeout)
	defer cancel()
	code, err := a.Execute(ctx, agent.Expand(tmpl, intent, proposalDir), proposalDir, f)
	progress.Stop()
	if err != nil {
		outcome := "failed"
		if errors.Is(err, context.DeadlineExceeded) {
			outcome = "timed_out"
		} else if errors.Is(err, context.Canceled) {
			outcome = "interrupted"
		}
		if rerr := record(outcome, nil, nil, ""); rerr != nil {
			return fail(errOut, rerr)
		}
		return fail(errOut, fmt.Errorf("baseline %s — log: %s", outcome, logPath))
	}
	if code != 0 {
		if rerr := record("failed", nil, nil, ""); rerr != nil {
			return fail(errOut, rerr)
		}
		return fail(errOut, fmt.Errorf("baseline failed (exit %d) — log: %s", code, logPath))
	}
	after, ignored, err := collectProposal(proposalDir, master)
	if err != nil {
		if rerr := record("failed", nil, nil, ""); rerr != nil {
			return fail(errOut, rerr)
		}
		return fail(errOut, fmt.Errorf("invalid split proposal — log: %s: %w", logPath, err))
	}
	for _, ig := range ignored {
		fmt.Fprintf(out, "warning: ignoring unexpected proposal file %s\n", ig)
	}
	propBefore, err := proposalBefore(w.root, after)
	if err != nil {
		return fail(errOut, err)
	}
	encoded, err := json.Marshal(splitProposal{Dir: proposalRel, Before: propBefore, After: after})
	if err != nil {
		return fail(errOut, err)
	}
	if err := record("proposed", after[0].Content, encoded, spec.HashFiles(after)); err != nil {
		return fail(errOut, err)
	}
	fmt.Fprintf(out, "split baseline proposed (%d files) via %s — log: %s\n", len(after), name, logRel)
	fmt.Fprintln(out, "review with `respex diff --baseline latest`, then `respex baseline accept` or `respex baseline discard`")
	return 0
}

func runBaseline(args []string, out, errOut io.Writer) int {
	fs := newFlagSet("baseline", errOut)
	intent := fs.String("intent", "", "additional project intent for the agent")
	merge := fs.Bool("merge", false, "merge repository findings into an existing spec")
	split := fs.Bool("split", false, "propose a multi-file spec bundle under specs/")
	noProgress := fs.Bool("no-progress", false, "disable the interactive progress indicator")
	if err := fs.Parse(args); err != nil {
		return fail(errOut, err)
	}
	usage := "respex baseline [--split] [--intent text] [--merge] [--no-progress] | respex baseline <accept|discard>"
	if fs.NArg() > 1 || (fs.NArg() == 1 && fs.Arg(0) != "accept" && fs.Arg(0) != "discard") {
		return fail(errOut, fmt.Errorf("unexpected argument %q — usage: %s", fs.Arg(0), usage))
	}
	w, err := discover()
	if err != nil {
		return fail(errOut, err)
	}
	lock, locked, err := tryApplyLock(filepath.Join(w.root, ".respex", "operation.lock"))
	if err != nil {
		return fail(errOut, fmt.Errorf("acquire operation lock: %w", err))
	}
	if !locked {
		return fail(errOut, errors.New("another apply, baseline, refine, restore, or edit is already running in this project"))
	}
	defer lock.Close()
	st, err := w.openState()
	if err != nil {
		return fail(errOut, err)
	}
	defer st.Close()
	if fs.NArg() == 1 {
		if fs.Arg(0) == "accept" {
			return runBaselineAccept(w, st, out, errOut)
		}
		return runBaselineDiscard(w, st, out, errOut)
	}
	pending, err := pendingProposal(st)
	if err != nil {
		return fail(errOut, err)
	}
	if pending != nil {
		return fail(errOut, errors.New("a split baseline proposal is pending — run `respex baseline accept` or `respex baseline discard`"))
	}
	before, err := spec.Read(w.specPath())
	if err != nil {
		return fail(errOut, err)
	}
	if spec.Hash(before) != spec.Hash([]byte(spec.Skeleton)) && !*merge {
		hint := "`respex baseline --merge`"
		if *split {
			hint = "`respex baseline --split --merge`"
		}
		return fail(errOut, fmt.Errorf("the spec contains meaningful content — review it and rerun with %s", hint))
	}
	if *split {
		return runBaselineSplit(w, st, *intent, *merge, *noProgress, before, out, errOut)
	}
	a, name, err := w.adapter()
	if err != nil {
		return fail(errOut, err)
	}
	candidatePath, cleanupCandidate, err := createSpecCandidate(w.root, before)
	if err != nil {
		return fail(errOut, err)
	}
	defer cleanupCandidate()
	logsDir := filepath.Join(w.root, ".respex", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fail(errOut, err)
	}
	f, err := os.CreateTemp(logsDir, time.Now().UTC().Format("20060102T150405Z")+"-baseline-*.log")
	if err != nil {
		return fail(errOut, err)
	}
	defer f.Close()
	logPath := f.Name()
	logRel := filepath.Join(".respex", "logs", filepath.Base(logPath))
	started := time.Now()
	record := func(outcome string, after []byte) error {
		if after == nil {
			after = []byte{}
		}
		afterHash := ""
		if len(after) > 0 {
			afterHash = spec.Hash(after)
		}
		_, err := st.InsertBaseline(name, outcome, spec.Hash(before), afterHash, before, after, nil, logRel, started, time.Now())
		return err
	}
	tmpl := agent.PromptBaseline
	if w.cfg.Prompts.Baseline != "" {
		tmpl = w.cfg.Prompts.Baseline
	}
	if *merge {
		tmpl += "\n\nMerge findings into the existing specification. Preserve established user intent and explicitly documented requirements unless they directly contradict observed behavior; record conflicts in Open Questions."
	}
	absSpec := candidatePath
	label := fmt.Sprintf("baselining repository via %s", name)
	progressEnabled := ui.IsTTY(out) && !*noProgress && os.Getenv("NO_COLOR") == ""
	if !progressEnabled {
		fmt.Fprintf(out, "%s; output: %s\n", label, logRel)
	}
	progress := ui.StartProgress(out, label, progressEnabled)
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, w.cfg.AgentTimeout)
	defer cancel()
	code, err := a.Execute(ctx, agent.Expand(tmpl, *intent, absSpec), absSpec, f)
	progress.Stop()
	if err != nil {
		outcome := "failed"
		if errors.Is(err, context.DeadlineExceeded) {
			outcome = "timed_out"
		} else if errors.Is(err, context.Canceled) {
			outcome = "interrupted"
		}
		if rerr := record(outcome, before); rerr != nil {
			return fail(errOut, rerr)
		}
		return fail(errOut, fmt.Errorf("baseline %s — log: %s", outcome, logPath))
	}
	if code != 0 {
		if err := record("failed", before); err != nil {
			return fail(errOut, err)
		}
		return fail(errOut, fmt.Errorf("baseline failed (exit %d) — log: %s", code, logPath))
	}
	after, err := installSpecCandidate(w.specPath(), candidatePath, before)
	if err != nil {
		candidate, _ := os.ReadFile(candidatePath)
		if rerr := record("failed", candidate); rerr != nil {
			return fail(errOut, rerr)
		}
		return fail(errOut, fmt.Errorf("install baseline spec — log: %s: %w", logPath, err))
	}
	warnMissingSections(out, after)
	outcome := "generated"
	if spec.Hash(before) == spec.Hash(after) {
		outcome = "unchanged"
	} else if *merge {
		outcome = "merged"
	}
	if err := record(outcome, after); err != nil {
		return fail(errOut, err)
	}
	fmt.Fprintf(out, "baseline %s via %s (%s… → %s…) — log: %s\n", outcome, name, spec.Hash(before)[:8], spec.Hash(after)[:8], logRel)
	fmt.Fprintln(out, "review with `respex diff --baseline latest`, then commit")
	return 0
}
