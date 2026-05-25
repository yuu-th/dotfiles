// Package identity classifies windows by identity strength. design.md §4-§7.
package identity

import (
	"sort"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// Class is the identity classification for resolving Desired→Live windows.
type Class string

const (
	ClassUniqueStrong Class = "unique-strong"
	ClassAmbiguous    Class = "ambiguous"
	ClassWeak         Class = "weak"
	ClassStale        Class = "stale"
	ClassMissing      Class = "missing"
)

// Resolution describes the result of resolving a DesiredWindowID.
type Resolution struct {
	Class                 Class
	Live                  w.LiveWindowID // valid only when ClassUniqueStrong
	Candidates            []w.LiveWindowID
	Confidence            float64
	Evidence              []MatchEvidence
	MissingEvidence       []RequiredEvidence
	ForbiddenEvidenceUsed []MatchEvidence
}

type MatchEvidence struct {
	Kind     string
	Strength w.MatchConfidence
	Window   w.LiveWindowID
	Value    string
}

type RequiredEvidence string

const (
	RequiredKind       RequiredEvidence = "kind"
	RequiredBundleID   RequiredEvidence = "bundle-id"
	RequiredExactTitle RequiredEvidence = "exact-title"
	RequiredWorkspace  RequiredEvidence = "workspace"
)

type ResolveOptions struct {
	ExpectedWorkspace w.WorkspaceID
}

// IdentifyWinnerAndOrphans realises SSOT §2.5 EC4 / §3.4 INV-01: when multiple
// live windows match the same desired identity, the most-recently-focused
// one is the canonical "正" and the rest become orphan candidates for cockpit
// [INVARIANT] card notification.
//
// Tiebreak policy (deterministic):
//   1. If observed.Focus.Window is among candidates → it's the winner.
//   2. Otherwise the lexicographically smallest LiveWindowID wins (candidates
//      are already sorted by Resolve).
//
// orphans contains every candidate that is NOT the winner.
func IdentifyWinnerAndOrphans(candidates []w.LiveWindowID, focused w.LiveWindowID) (winner w.LiveWindowID, orphans []w.LiveWindowID) {
	if len(candidates) == 0 {
		return "", nil
	}
	winner = candidates[0]
	for _, c := range candidates {
		if c == focused {
			winner = c
			break
		}
	}
	for _, c := range candidates {
		if c != winner {
			orphans = append(orphans, c)
		}
	}
	return winner, orphans
}

// ResolveWithFocusTiebreak applies INV-01 tiebreak after a normal Resolve:
// converts ClassAmbiguous to ClassUniqueStrong by picking the focused (or
// smallest) candidate as the live winner. The full candidate list remains
// in res.Candidates so callers can compute the orphan set.
func ResolveWithFocusTiebreak(desired w.DesiredWindow, observed w.ObservedWorld, opts ResolveOptions) Resolution {
	res := ResolveWithOptions(desired, observed, opts)
	if res.Class != ClassAmbiguous || len(res.Candidates) == 0 {
		return res
	}
	winner, _ := IdentifyWinnerAndOrphans(res.Candidates, observed.Focus.Window)
	res.Live = winner
	res.Class = ClassUniqueStrong
	return res
}

// Resolve classifies a DesiredWindowID against the ObservedWorld using TitleContract + match hints.
// Pure function; iteration is sorted by LiveWindowID for determinism.
func Resolve(desired w.DesiredWindow, observed w.ObservedWorld) Resolution {
	return ResolveWithOptions(desired, observed, ResolveOptions{})
}

func ResolveWithOptions(desired w.DesiredWindow, observed w.ObservedWorld, opts ResolveOptions) Resolution {
	keys := make([]w.LiveWindowID, 0, len(observed.Windows))
	for k := range observed.Windows {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	var strong []w.LiveWindowID
	var weak []w.LiveWindowID
	var stale []w.LiveWindowID
	var evidence []MatchEvidence
	var forbidden []MatchEvidence
	missing := requiredEvidence(desired, opts)

	for _, k := range keys {
		ow := observed.Windows[k]
		// Kind must match. Viewer mirrors share DesiredWindowID with their AI source but
		// have Kind=WindowViewer; only same-kind windows are candidates.
		if ow.Kind != desired.Kind {
			continue
		}
		missing = removeRequired(missing, RequiredKind)
		if opts.ExpectedWorkspace != "" {
			if ow.Workspace != opts.ExpectedWorkspace {
				if candidateHasIdentityLink(desired, ow) {
					forbidden = append(forbidden, MatchEvidence{Kind: "workspace-mismatch", Strength: w.MatchStrong, Window: k, Value: string(ow.Workspace)})
				}
				if matchedToDesired(desired, ow) {
					stale = append(stale, k)
				}
				continue
			}
			evidence = append(evidence, MatchEvidence{Kind: "workspace", Strength: w.MatchStrong, Window: k, Value: string(ow.Workspace)})
			missing = removeRequired(missing, RequiredWorkspace)
		}
		if desired.App.BundleID != "" {
			if ow.App.BundleID != desired.App.BundleID {
				if matchesTitleContract(desired, ow) || matchedToDesired(desired, ow) {
					forbidden = append(forbidden, MatchEvidence{Kind: "bundle-id-mismatch", Strength: w.MatchWeak, Window: k, Value: ow.App.BundleID})
				}
				if matchedToDesired(desired, ow) {
					stale = append(stale, k)
				}
				continue
			}
			evidence = append(evidence, MatchEvidence{Kind: "bundle-id", Strength: w.MatchStrong, Window: k, Value: ow.App.BundleID})
			missing = removeRequired(missing, RequiredBundleID)
		}
		if desired.TitleContract.Authority == w.TitleControllerOwned &&
			desired.TitleContract.Expected != "" {
			if ow.Title.Value != desired.TitleContract.Expected {
				if matchedToDesired(desired, ow) {
					stale = append(stale, k)
					evidence = append(evidence, MatchEvidence{Kind: "matched-to-stale-title", Strength: w.MatchStrong, Window: k, Value: ow.Title.Value})
				}
				continue
			}
			evidence = append(evidence, MatchEvidence{Kind: "exact-title", Strength: w.MatchStrong, Window: k, Value: desired.TitleContract.Expected})
			missing = removeRequired(missing, RequiredExactTitle)
		}
		// app-owned title with an Expected value (e.g. Zed where the
		// title equals the project path basename): use it as a hard
		// disambiguator so multi-window single-PID apps don't all
		// match the same Desired entry by bundle-id alone.
		if desired.TitleContract.Authority == w.TitleAppOwned &&
			desired.TitleContract.Expected != "" {
			if ow.Title.Value != desired.TitleContract.Expected {
				if matchedToDesired(desired, ow) {
					stale = append(stale, k)
					evidence = append(evidence, MatchEvidence{Kind: "matched-to-stale-title", Strength: w.MatchStrong, Window: k, Value: ow.Title.Value})
				}
				continue
			}
			// Bug 2026-05-19: previously this branch only *disqualified*
			// title mismatches; an exact-title match for an AppOwned
			// contract was treated as "not disqualifying" and then fell
			// through to the no-evidence default — landing in
			// ClassMissing. That left Zed (or any AppOwned editor whose
			// window title equals the project basename) permanently
			// un-identifiable, so the planner re-emitted spawn-editor
			// every reconcile cycle and unarchive never converged.
			// Treat exact-title + matching bundleID as strong evidence,
			// matching the symmetric ControllerOwned branch below.
			strong = append(strong, k)
			evidence = append(evidence, MatchEvidence{Kind: "exact-title", Strength: w.MatchStrong, Window: k, Value: desired.TitleContract.Expected})
			missing = removeRequired(missing, RequiredExactTitle)
			continue
		}
		// Direct controller-set match wins.
		if matchedToDesired(desired, ow) {
			strong = append(strong, k)
			evidence = append(evidence, MatchEvidence{Kind: "matched-to", Strength: w.MatchStrong, Window: k, Value: string(desired.ID.Project)})
			continue
		}
		// Title contract: controller-owned exact match.
		if desired.TitleContract.Authority == w.TitleControllerOwned &&
			desired.TitleContract.Expected != "" &&
			ow.Title.Value == desired.TitleContract.Expected {
			strong = append(strong, k)
			continue
		}
		if desired.TitleContract.Authority == w.TitlePrefixOwned &&
			desired.TitleContract.Prefix != "" &&
			startsWith(ow.Title.Value, desired.TitleContract.Prefix) {
			weak = append(weak, k)
			evidence = append(evidence, MatchEvidence{Kind: "title-prefix", Strength: w.MatchWeak, Window: k, Value: desired.TitleContract.Prefix})
			continue
		}
		// Match hints.
		for _, h := range desired.MatchHints {
			if h.Kind == w.MatchByBundleID &&
				desired.TitleContract.Authority == w.TitleControllerOwned &&
				desired.TitleContract.Expected != "" &&
				ow.Title.Value != desired.TitleContract.Expected {
				continue
			}
			if h.Confidence == w.MatchStrong && hintMatches(h, ow) {
				strong = append(strong, k)
				evidence = append(evidence, MatchEvidence{Kind: string(h.Kind), Strength: w.MatchStrong, Window: k, Value: h.Pattern})
				break
			}
			if h.Confidence == w.MatchWeak && hintMatches(h, ow) {
				weak = append(weak, k)
				evidence = append(evidence, MatchEvidence{Kind: string(h.Kind), Strength: w.MatchWeak, Window: k, Value: h.Pattern})
				break
			}
		}
	}

	switch {
	case len(strong) == 1:
		return Resolution{Class: ClassUniqueStrong, Live: strong[0], Candidates: strong, Confidence: 1.0, Evidence: evidenceForWindow(evidence, strong[0]), MissingEvidence: nil, ForbiddenEvidenceUsed: forbidden}
	case len(strong) > 1:
		return Resolution{Class: ClassAmbiguous, Candidates: strong, Confidence: 0.0, Evidence: evidence, MissingEvidence: missing, ForbiddenEvidenceUsed: forbidden}
	case len(weak) == 1:
		return Resolution{Class: ClassWeak, Candidates: weak, Confidence: 0.5, Evidence: evidenceForWindow(evidence, weak[0]), MissingEvidence: missing, ForbiddenEvidenceUsed: forbidden}
	case len(weak) > 1:
		return Resolution{Class: ClassAmbiguous, Candidates: weak, Confidence: 0.0, Evidence: evidence, MissingEvidence: missing, ForbiddenEvidenceUsed: forbidden}
	case len(stale) > 0:
		return Resolution{Class: ClassStale, Candidates: stale, Confidence: 0.0, Evidence: evidence, MissingEvidence: missing, ForbiddenEvidenceUsed: forbidden}
	default:
		return Resolution{Class: ClassMissing, MissingEvidence: missing, ForbiddenEvidenceUsed: forbidden}
	}
}

func requiredEvidence(desired w.DesiredWindow, opts ResolveOptions) []RequiredEvidence {
	req := []RequiredEvidence{RequiredKind}
	if opts.ExpectedWorkspace != "" {
		req = append(req, RequiredWorkspace)
	}
	if desired.App.BundleID != "" {
		req = append(req, RequiredBundleID)
	}
	if desired.TitleContract.Authority == w.TitleControllerOwned && desired.TitleContract.Expected != "" {
		req = append(req, RequiredExactTitle)
	}
	return req
}

func removeRequired(req []RequiredEvidence, item RequiredEvidence) []RequiredEvidence {
	out := req[:0]
	for _, r := range req {
		if r != item {
			out = append(out, r)
		}
	}
	return out
}

func matchesTitleContract(desired w.DesiredWindow, ow w.ObservedWindow) bool {
	switch desired.TitleContract.Authority {
	case w.TitleControllerOwned:
		return desired.TitleContract.Expected != "" && ow.Title.Value == desired.TitleContract.Expected
	case w.TitlePrefixOwned:
		return desired.TitleContract.Prefix != "" && startsWith(ow.Title.Value, desired.TitleContract.Prefix)
	}
	return false
}

func matchedToDesired(desired w.DesiredWindow, ow w.ObservedWindow) bool {
	return ow.MatchedTo != nil && *ow.MatchedTo == desired.ID
}

func candidateHasIdentityLink(desired w.DesiredWindow, ow w.ObservedWindow) bool {
	return matchedToDesired(desired, ow) || matchesTitleContract(desired, ow) || (desired.App.BundleID != "" && ow.App.BundleID == desired.App.BundleID)
}

func evidenceForWindow(all []MatchEvidence, id w.LiveWindowID) []MatchEvidence {
	out := []MatchEvidence{}
	for _, ev := range all {
		if ev.Window == id {
			out = append(out, ev)
		}
	}
	return out
}

func startsWith(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

func hintMatches(h w.MatchHint, ow w.ObservedWindow) bool {
	switch h.Kind {
	case w.MatchByTitlePrefix:
		return startsWith(ow.Title.Value, h.Pattern)
	case w.MatchByBundleID:
		return ow.App.BundleID == h.Pattern
	}
	return false
}

// PopulateMatchedTo runs Resolve over every project's DesiredWindows
// and stamps the resulting strong-match LiveWindow's MatchedTo back to
// the originating DesiredWindowID. Returns a shallow-copied
// ObservedWorld so callers can replace state.Observed atomically.
//
// Why this exists (2026-05-19): the Tier-1 orphan detector in
// reducer.ReactToEvent (§3.5) skips a live window only when its
// MatchedTo is non-nil; every other call-site (planner / semop /
// executor) re-runs identity.Resolve on the fly. Production code never
// wrote MatchedTo, so every managed-kind window was perpetually
// re-detected as an orphan even when identity.Resolve would have
// unambiguously matched it — producing spurious "manual ai window"
// cards. Centralising MatchedTo population here gives all consumers a
// consistent, resolver-truthful hint without changing the on-the-fly
// Resolve path the planner/executor already trust.
//
// Stale-class resolutions (matchedToDesired() returning true but title
// drift) intentionally do NOT overwrite an existing MatchedTo — they
// instead leave the cached value so the planner's "stale-title revert"
// logic still fires.
func PopulateMatchedTo(desired w.DesiredWorld, observed w.ObservedWorld) w.ObservedWorld {
	if len(observed.Windows) == 0 {
		return observed
	}
	// Shallow-copy Windows so the returned value is owner-independent
	// of the input. We do NOT pre-clear MatchedTo: callers that already
	// hold authoritative hints (the simulator + fake adapters; a
	// previous cycle's stamp that the resolver still agrees with) must
	// survive — our job is to fill GAPS, not to litigate every
	// pre-existing entry. The resolver is the tiebreaker only when a
	// live window has no MatchedTo yet.
	out := observed
	out.Windows = make(map[w.LiveWindowID]w.ObservedWindow, len(observed.Windows))
	for k, v := range observed.Windows {
		out.Windows[k] = v
	}
	pids := make([]w.ProjectID, 0, len(desired.Projects))
	for k := range desired.Projects {
		pids = append(pids, k)
	}
	sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })

	// Pass 1: build the candidate map live → []DesiredWindowID. A live
	// window claimed by more than one desired window is ambiguous and
	// MUST NOT be stamped, otherwise we hand the orphan detector / the
	// invariant checks a value the resolver itself could not commit
	// to. (Bug 2026-05-19: weak prefix-owned titles can make the same
	// live window resolve to multiple desired windows. A naive
	// single-pass stamp picks the first sorted project — typically an
	// archived one — and tricks invariant 5 "archived-absent" into
	// firing, blocking lifecycle.)
	candidates := map[w.LiveWindowID][]w.DesiredWindowID{}
	for _, pid := range pids {
		p := desired.Projects[pid]
		for _, dw := range p.Windows {
			res := Resolve(dw, out)
			if res.Class != ClassUniqueStrong && res.Class != ClassWeak {
				continue
			}
			live := res.Live
			if live == "" && len(res.Candidates) == 1 {
				live = res.Candidates[0]
			}
			if live == "" {
				continue
			}
			candidates[live] = append(candidates[live], dw.ID)
		}
	}

	// Pass 2: stamp only when the candidate set is exactly one. Each
	// live window's MatchedTo is the resolver's unique commitment, or
	// nil when the resolver itself is non-unique. Existing non-nil
	// MatchedTo from the input is preserved (simulator / fake hint).
	for live, ids := range candidates {
		if len(ids) != 1 {
			continue
		}
		ow := out.Windows[live]
		if ow.MatchedTo != nil {
			continue
		}
		id := ids[0]
		ow.MatchedTo = &id
		out.Windows[live] = ow
	}
	return out
}

// ClassifyAll resolves every DesiredWindow of every active project. Useful for invariant checks.
// Returns map keyed by the canonical desired-window key.
func ClassifyAll(state w.WorldState) map[w.DesiredWindowID]Resolution {
	out := make(map[w.DesiredWindowID]Resolution)
	// iterate sorted project IDs.
	pids := make([]w.ProjectID, 0, len(state.Desired.Projects))
	for k := range state.Desired.Projects {
		pids = append(pids, k)
	}
	sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
	for _, pid := range pids {
		p := state.Desired.Projects[pid]
		for _, dw := range p.Windows {
			out[dw.ID] = Resolve(dw, state.Observed)
		}
	}
	return out
}
