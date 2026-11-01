package service

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	datePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?:^|[^0-9])([0-9]{4})[-/]([0-9]{1,2})[-/]([0-9]{1,2})(?:$|[^0-9])`),
		regexp.MustCompile(`(?:^|[^0-9])([0-9]{4})年([0-9]{1,2})月([0-9]{1,2})日?(?:$|[^0-9])`),
		regexp.MustCompile(`(?:^|[^0-9])([0-9]{4})([0-9]{2})([0-9]{2})(?:$|[^0-9])`),
	}
	amountTokenPattern = regexp.MustCompile(`[¥￥$]?\s*[0-9][0-9,]*(?:\.[0-9]+)?\s*(?:元|人民币)?`)
)

// parseOCRText only emits candidates when amount, direction, and date are unambiguous.
// It joins a short receipt neighborhood to support OCR output split across lines.
func parseOCRText(text string) []OCRCandidate {
	lines := strings.FieldsFunc(normalizeOCR(text), func(r rune) bool { return r == '\n' || r == '\r' || r == ';' })
	candidates := make([]OCRCandidate, 0)
	seen := make(map[string]struct{})
	for i := range lines {
		start, end := i-1, i+1
		if start < 0 {
			start = 0
		}
		if end >= len(lines) {
			end = len(lines) - 1
		}
		window := strings.TrimSpace(strings.Join(lines[start:end+1], " "))
		if len([]rune(window)) > 1000 {
			continue
		}
		lower := strings.ToLower(window)
		// 方向关键词（QA Round2）：平台名（微信支付/支付宝）本身不代表收支方向，
		// 只作为修饰词出现在 income 短语里（"微信支付收入"/"支付宝收入"）。
		// 把平台名放进 expense 会让收入文案同时命中两个方向而被丢弃。
		income := hasAny(lower, "收入", "入账", "收款", "到账", "转入")
		expense := hasAny(lower, "支出", "付款", "消费", "转出", "扣款")
		if income == expense {
			continue
		}
		dates := extractDates(window)
		if len(dates) != 1 {
			continue
		}
		amounts := extractAmounts(withoutDates(window))
		if len(amounts) != 1 {
			continue
		}
		direction := "expense"
		if income {
			direction = "income"
		}
		candidate := OCRCandidate{AmountCents: amounts[0], Direction: direction, OccurredDate: dates[0], Description: window, Confidence: 0}
		key := fmt.Sprintf("%s:%d:%s", candidate.Direction, candidate.AmountCents, candidate.OccurredDate)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func normalizeOCR(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '０' && r <= '９':
			r = '0' + (r - '０')
		case r == '，':
			r = ','
		case r == '．':
			r = '.'
		case r == '：':
			r = ':'
		case r == '－':
			r = '-'
		}
		b.WriteRune(r)
	}
	return b.String()
}

func hasAny(s string, words ...string) bool {
	for _, word := range words {
		if strings.Contains(s, word) {
			return true
		}
	}
	return false
}

func extractAmounts(s string) []int64 {
	matches := amountTokenPattern.FindAllStringIndex(s, -1)
	amounts := make([]int64, 0, len(matches))
	for _, loc := range matches {
		// 符号边界（QA Round2）：token 左侧紧邻 +/- 时说明这是带符号的数（如 "-1.00"），
		// regex 只捕到数字部分会把负数误判成正数——保守契约要求拒绝，绝不取绝对值。
		if loc[0] > 0 && (s[loc[0]-1] == '-' || s[loc[0]-1] == '+') {
			return nil
		}
		match := s[loc[0]:loc[1]]
		trimmed := strings.TrimSpace(match)
		if trimmed == "" {
			continue
		}
		value := strings.TrimSpace(strings.TrimLeft(trimmed, "¥￥$"))
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(value, "人民币"), "元"))
		if value == "" || strings.ContainsAny(value, "-+eE") {
			return nil
		}
		if !validGroupedAmount(value) {
			return nil
		}
		value = strings.ReplaceAll(value, ",", "")
		parts := strings.Split(value, ".")
		if len(parts) > 2 || (len(parts) == 2 && (len(parts[1]) < 1 || len(parts[1]) > 2)) {
			return nil
		}
		cents := parts[0] + "00"
		if len(parts) == 2 {
			cents = parts[0] + parts[1]
			if len(parts[1]) == 1 {
				cents += "0"
			}
		}
		n, err := strconv.ParseInt(cents, 10, 64)
		if err != nil || n <= 0 {
			return nil
		}
		amounts = append(amounts, n)
	}
	return amounts
}

func validGroupedAmount(value string) bool {
	parts := strings.Split(value, ".")
	whole := parts[0]
	if whole == "" {
		return false
	}
	if strings.Contains(whole, ",") {
		groups := strings.Split(whole, ",")
		if len(groups[0]) < 1 || len(groups[0]) > 3 {
			return false
		}
		for _, group := range groups {
			if group == "" {
				return false
			}
			for _, r := range group {
				if r < '0' || r > '9' {
					return false
				}
			}
		}
		for _, group := range groups[1:] {
			if len(group) != 3 {
				return false
			}
		}
	} else {
		for _, r := range whole {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func withoutDates(s string) string {
	for _, pattern := range datePatterns {
		s = pattern.ReplaceAllString(s, " ")
	}
	return s
}

func extractDates(s string) []string {
	cleaned := s
	dates := make([]string, 0, 2)
	for _, pattern := range datePatterns {
		for _, match := range pattern.FindAllStringSubmatch(cleaned, -1) {
			if len(match) != 4 {
				continue
			}
			y, e1 := strconv.Atoi(match[1])
			m, e2 := strconv.Atoi(match[2])
			d, e3 := strconv.Atoi(match[3])
			if e1 != nil || e2 != nil || e3 != nil || y < 2000 || y > 2100 || !validDate(y, time.Month(m), d) {
				return nil
			}
			dates = append(dates, fmt.Sprintf("%04d-%02d-%02d", y, m, d))
		}
	}
	return dates
}

func extractDate(s string) string {
	dates := extractDates(s)
	if len(dates) != 1 {
		return ""
	}
	return dates[0]
}

func validDate(y int, m time.Month, d int) bool {
	if m < 1 || m > 12 || d < 1 {
		return false
	}
	date := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return date.Day() == d && date.Month() == m
}
