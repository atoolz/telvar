package scorecard

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ahlert/telvar/internal/catalog"
	"github.com/ahlert/telvar/internal/config"
)

func EvaluateRule(rule config.ScorecardRule, files catalog.FileChecker, entity *catalog.Entity) catalog.RuleResult {
	result := catalog.RuleResult{
		Name:   rule.Name,
		Weight: rule.Weight,
	}

	if result.Weight == 0 {
		result.Weight = 1
	}

	check := strings.TrimSpace(rule.Check)

	passed, detail, err := evalExpression(check, files, entity)
	if err != nil {
		result.Passed = false
		result.Detail = fmt.Sprintf("error: %s", err)
		return result
	}

	result.Passed = passed
	result.Detail = detail
	return result
}

func evalExpression(expr string, files catalog.FileChecker, entity *catalog.Entity) (bool, string, error) {
	expr = strings.TrimSpace(expr)

	if strings.Contains(expr, " || ") {
		parts := strings.SplitN(expr, " || ", 2)
		leftOk, _, err := evalExpression(parts[0], files, entity)
		if err != nil {
			return false, "", err
		}
		if leftOk {
			return true, "", nil
		}
		return evalExpression(parts[1], files, entity)
	}

	if strings.Contains(expr, " && ") {
		parts := strings.SplitN(expr, " && ", 2)
		leftOk, leftDetail, err := evalExpression(parts[0], files, entity)
		if err != nil {
			return false, "", err
		}
		if !leftOk {
			return false, leftDetail, nil
		}
		return evalExpression(parts[1], files, entity)
	}

	if strings.Contains(expr, " == ") {
		return evalComparison(expr, " == ", files, entity)
	}
	if strings.Contains(expr, " != ") {
		return evalComparison(expr, " != ", files, entity)
	}
	if strings.Contains(expr, " > ") {
		return evalNumericComparison(expr, " > ", files, entity)
	}
	if strings.Contains(expr, " < ") {
		return evalNumericComparison(expr, " < ", files, entity)
	}

	return evalCall(expr, files, entity)
}

func evalComparison(expr, op string, files catalog.FileChecker, entity *catalog.Entity) (bool, string, error) {
	parts := strings.SplitN(expr, op, 2)
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])

	rightNum, parseErr := strconv.Atoi(right)
	if parseErr == nil {
		leftNum := resolveNumeric(left, entity)
		detail := fmt.Sprintf("%s = %d", left, leftNum)
		switch op {
		case " == ":
			return leftNum == rightNum, detail, nil
		case " != ":
			return leftNum != rightNum, detail, nil
		default:
			return false, detail, fmt.Errorf("unsupported comparison operator: %s", op)
		}
	}

	leftVal, _, err := evalCall(left, files, entity)
	if err != nil {
		return false, "", err
	}

	rightBool := right == "true"
	switch op {
	case " == ":
		return leftVal == rightBool, "", nil
	case " != ":
		return leftVal != rightBool, "", nil
	default:
		return false, "", fmt.Errorf("unsupported comparison operator: %s", op)
	}
}

func evalNumericComparison(expr, op string, files catalog.FileChecker, entity *catalog.Entity) (bool, string, error) {
	parts := strings.SplitN(expr, op, 2)
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])

	leftVal := resolveNumeric(left, entity)
	rightVal, err := strconv.Atoi(right)
	if err != nil {
		return false, "", fmt.Errorf("invalid number: %s", right)
	}

	detail := fmt.Sprintf("%s = %d", left, leftVal)
	switch op {
	case " > ":
		return leftVal > rightVal, detail, nil
	case " < ":
		return leftVal < rightVal, detail, nil
	default:
		return false, detail, fmt.Errorf("unknown operator: %s", op)
	}
}

func evalCall(expr string, files catalog.FileChecker, entity *catalog.Entity) (bool, string, error) {
	expr = strings.TrimSpace(expr)

	if strings.HasPrefix(expr, "has_file(") && strings.HasSuffix(expr, ")") {
		arg := extractArg(expr)
		if strings.Contains(arg, "*") {
			return files.HasFileGlob(arg), "", nil
		}
		return files.HasFile(arg), "", nil
	}

	if strings.HasPrefix(expr, "cve_count(") && strings.HasSuffix(expr, ")") {
		severity := extractArg(expr)
		key := "vuln." + severity
		val := entity.Metadata[key]
		num, _ := strconv.Atoi(val)
		// Standalone cve_count() means "no CVEs" (passes when count is 0).
		// For threshold checks, use cve_count("x") == 0 or cve_count("x") < 5.
		return num == 0, fmt.Sprintf("%s = %d", key, num), nil
	}

	if strings.HasPrefix(expr, "has_topic(") && strings.HasSuffix(expr, ")") {
		topic := extractArg(expr)
		topics := entity.Tags["topics"]
		for _, t := range strings.Split(topics, ",") {
			if strings.TrimSpace(t) == topic {
				return true, "", nil
			}
		}
		return false, "", nil
	}

	if expr == "owner_set()" {
		return entity.Owner != "", "", nil
	}

	if strings.HasPrefix(expr, "language_is(") && strings.HasSuffix(expr, ")") {
		lang := extractArg(expr)
		return entity.Language == lang, "", nil
	}

	return false, fmt.Sprintf("unknown check: %s", expr), fmt.Errorf("unknown check: %s", expr)
}

func extractArg(call string) string {
	start := strings.Index(call, "(")
	end := strings.LastIndex(call, ")")
	if start < 0 || end < 0 {
		return ""
	}
	arg := call[start+1 : end]
	arg = strings.Trim(arg, `"'`)
	return arg
}

func resolveNumeric(expr string, entity *catalog.Entity) int {
	if strings.HasPrefix(expr, "cve_count(") {
		severity := extractArg(expr)
		key := "vuln." + severity
		val := entity.Metadata[key]
		num, _ := strconv.Atoi(val)
		return num
	}
	return 0
}
