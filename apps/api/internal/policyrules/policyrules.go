// Package policyrules evaluates typed policy rules against a request's structured
// details, producing exact pass/warn/fail checks. It is pure (no DB, no LLM) so
// the orchestration engine and the demo seed can both use it and get identical
// results offline.
package policyrules

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Rule is one machine-checkable condition on a request detail field.
//
//	field: a key in the request's details (e.g. "total_cost")
//	op:    lte | gte | lt | gt | eq | ne | exists
//	value: the threshold to compare against
//	severity: info | warning | critical — how bad a failure is
//	message:  shown when the rule fails
type Rule struct {
	Label    string `json:"label"`
	Field    string `json:"field"`
	Op       string `json:"op"`
	Value    any    `json:"value"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// Check is the result of evaluating one rule.
type Check struct {
	Label       string `json:"label"`
	Status      string `json:"status"` // pass | warn | fail
	Detail      string `json:"detail"`
	PolicyTitle string `json:"policy_title"`
}

// Evaluate runs a policy's rules against the request details. Rules whose field
// is absent are skipped (not applicable) — except the "exists" op. Returns one
// check per applicable rule, tagged with the policy title.
func Evaluate(policyTitle string, rules []Rule, details map[string]any) []Check {
	out := make([]Check, 0, len(rules))
	for _, r := range rules {
		dv, present := details[r.Field]
		op := strings.ToLower(strings.TrimSpace(r.Op))

		if op == "exists" {
			out = append(out, mkCheck(r, policyTitle, present, fmt.Sprintf("%q provided", r.Field)))
			continue
		}
		if !present {
			continue // not applicable to this request
		}

		var pass bool
		var detail string
		switch op {
		case "lte", "gte", "lt", "gt":
			a, aok := toFloat(dv)
			b, bok := toFloat(r.Value)
			if !aok || !bok {
				continue
			}
			pass = compareNum(a, op, b)
			detail = fmt.Sprintf("%s %s %s", fmtNum(a), symbol(op), fmtNum(b))
		case "eq":
			pass = strings.EqualFold(fmt.Sprint(dv), fmt.Sprint(r.Value))
			detail = fmt.Sprintf("%v = %v", dv, r.Value)
		case "ne":
			pass = !strings.EqualFold(fmt.Sprint(dv), fmt.Sprint(r.Value))
			detail = fmt.Sprintf("%v ≠ %v", dv, r.Value)
		default:
			continue
		}
		out = append(out, mkCheck(r, policyTitle, pass, detail))
	}
	return out
}

func mkCheck(r Rule, policyTitle string, pass bool, passDetail string) Check {
	label := r.Label
	if label == "" {
		label = r.Field
	}
	if pass {
		return Check{Label: label, Status: "pass", Detail: passDetail, PolicyTitle: policyTitle}
	}
	status := "warn"
	if strings.EqualFold(r.Severity, "critical") {
		status = "fail"
	}
	detail := r.Message
	if detail == "" {
		detail = passDetail
	}
	return Check{Label: label, Status: status, Detail: detail, PolicyTitle: policyTitle}
}

func compareNum(a float64, op string, b float64) bool {
	switch op {
	case "lte":
		return a <= b
	case "gte":
		return a >= b
	case "lt":
		return a < b
	case "gt":
		return a > b
	}
	return false
}

func symbol(op string) string {
	switch op {
	case "lte":
		return "≤"
	case "gte":
		return "≥"
	case "lt":
		return "<"
	case "gt":
		return ">"
	}
	return op
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(strings.ReplaceAll(x, ",", "")), 64)
		return f, err == nil
	}
	return 0, false
}

func fmtNum(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
