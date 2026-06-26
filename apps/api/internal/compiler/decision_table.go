package compiler

import (
	"regexp"
	"sort"
	"strings"

	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
)

// DecisionTable is a view over one exclusive gateway's outgoing
// flows, summarized as a row-per-branch table. A compiler (DMN,
// Elsa.Switch) can project this into its native shape; the web UI
// renders it inline on the Inspector when the user clicks the
// originating gateway.
//
// Detection is loose by design: the extractor emits JUEL-like
// strings and we tokenize them with regex rather than a full AST.
// This gets us 80% of the real-world patterns ("amount > N",
// "category == 'travel'", "approved == true") without building a
// parser for an LLM-generated syntax we don't fully control.
type DecisionTable struct {
	GatewayID string         `json:"gateway_id"`
	Variables []string       `json:"variables"`
	Rules     []DecisionRule `json:"rules"`
	Evidence  []string       `json:"evidence,omitempty"`
}

// DecisionRule is one outgoing branch: a map from variable name to
// the human-readable predicate on that variable plus the target the
// flow points at. Variables missing from the predicate map means
// "don't care" in DMN parlance — anything goes.
type DecisionRule struct {
	FlowID     string            `json:"flow_id"`
	Target     string            `json:"target"`
	Predicates map[string]string `json:"predicates"` // variable -> human expression
	IsDefault  bool              `json:"is_default"`
}

// Minimal threshold: consolidation only triggers when a gateway has
// at least three branches — fewer reads as a plain if/else and
// benefits more from explicit flow labels than from a table.
const minBranchesForTable = 3

// Variable references in JUEL/DMN-ish expressions. We capture
// identifiers that follow ${, or appear after common operators.
// operatorSplit requires at least one whitespace character around
// the word operators `and` / `or`, otherwise the substring `or`
// inside string literals like "travel" would split the expression
// mid-word.
var (
	variableRegex = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
	operatorSplit = regexp.MustCompile(`\s*&&\s*|\s*\|\|\s*|\s+and\s+|\s+or\s+`)
)

// Reserved tokens that the variable scanner should skip — they're
// operators/keywords, not variable names.
var reservedTokens = map[string]bool{
	"true": true, "false": true, "null": true,
	"and": true, "or": true, "not": true,
	"in": true, "contains": true,
}

// AnalyzeDecisionTables walks the workflow and returns one
// DecisionTable per exclusive gateway that meets the consolidation
// threshold. Order is deterministic (gateway id ascending).
func AnalyzeDecisionTables(wf *ir.Workflow) []DecisionTable {
	if wf == nil {
		return nil
	}

	// Map gateway id -> its outgoing flows, sorted by flow id.
	exclusive := map[string]bool{}
	for _, g := range wf.Gateways {
		if g.Type == "exclusive" {
			exclusive[g.ID] = true
		}
	}
	outFlows := map[string][]*ir.Flow{}
	for i := range wf.Flows {
		f := &wf.Flows[i]
		if exclusive[f.From] {
			outFlows[f.From] = append(outFlows[f.From], f)
		}
	}
	for id := range outFlows {
		sort.SliceStable(outFlows[id], func(i, j int) bool {
			return outFlows[id][i].ID < outFlows[id][j].ID
		})
	}

	// Sort gateway ids for deterministic output.
	gateIDs := make([]string, 0, len(outFlows))
	for id := range outFlows {
		gateIDs = append(gateIDs, id)
	}
	sort.Strings(gateIDs)

	var tables []DecisionTable
	for _, gid := range gateIDs {
		flows := outFlows[gid]
		if len(flows) < minBranchesForTable {
			continue
		}
		// Extract per-flow predicates grouped by variable.
		rulePredicates := make([]map[string]string, len(flows))
		allVariables := map[string]int{} // how many rules reference each
		for i, f := range flows {
			rulePredicates[i] = map[string]string{}
			if f.Condition == nil || f.Condition.Expression == "" {
				continue
			}
			forEachConjunct(f.Condition.Expression, func(varName, predicate string) {
				// Keep the most specific predicate per variable —
				// last-one-wins for simplicity; real DMN would need
				// proper intersection logic.
				rulePredicates[i][varName] = predicate
				allVariables[varName]++
			})
		}

		// Consolidation heuristic: at least one variable must appear
		// in a majority of the rules. Otherwise the "table" is just
		// a scattered if-else and reads better as individual edges.
		majority := (len(flows) + 1) / 2
		hasSharedVar := false
		for _, count := range allVariables {
			if count >= majority {
				hasSharedVar = true
				break
			}
		}
		if !hasSharedVar {
			continue
		}

		// Build sorted variable list (only those mentioned in ≥2
		// rules to keep the table compact).
		variables := []string{}
		for name, count := range allVariables {
			if count >= 2 {
				variables = append(variables, name)
			}
		}
		sort.Strings(variables)

		// Build rules.
		var rules []DecisionRule
		var evidence []string
		for i, f := range flows {
			isDefault := f.Condition == nil || f.Condition.Expression == ""
			rules = append(rules, DecisionRule{
				FlowID:     f.ID,
				Target:     f.To,
				Predicates: rulePredicates[i],
				IsDefault:  isDefault,
			})
			if !isDefault && f.Condition.Evidence != "" {
				evidence = append(evidence, f.Condition.Evidence)
			}
		}

		tables = append(tables, DecisionTable{
			GatewayID: gid,
			Variables: variables,
			Rules:     rules,
			Evidence:  evidence,
		})
	}
	return tables
}

// forEachConjunct splits a JUEL-ish boolean expression on && / and
// boundaries and calls cb(variable, predicate-as-string) for each
// top-level comparison. Bare `${...}` wrappers are stripped.
// Heuristic-only — handles `${amount > 50000}`,
// `${amount > 50000 && category == "travel"}`, and their &&/and
// equivalents. Parentheses and nested disjunctions are treated as
// single atoms.
func forEachConjunct(expr string, cb func(varName, predicate string)) {
	// Strip surrounding ${...} if present.
	s := strings.TrimSpace(expr)
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		s = s[2 : len(s)-1]
	}
	parts := operatorSplit.Split(s, -1)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// First identifier in the atom is our variable guess.
		first := firstIdentifier(p)
		if first == "" {
			continue
		}
		cb(first, p)
	}
}

// firstIdentifier returns the leftmost identifier in s that isn't
// a reserved keyword. Returns "" if nothing qualifies.
func firstIdentifier(s string) string {
	for _, m := range variableRegex.FindAllString(s, -1) {
		if !reservedTokens[strings.ToLower(m)] {
			return m
		}
	}
	return ""
}
