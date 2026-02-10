package domain

import (
	"regexp"
	"strings"
)

// Code 是作品的唯一主键（规范化后形如 CAWD-895）。
//
// 约束：要么得到唯一 Code，要么失败；宁可 unmatched，也不允许写错。
type Code string

var codeRE = regexp.MustCompile(`^[A-Z]{2,6}-[0-9]{2,5}$`)

// ParseCode 校验并解析规范化后的 CODE 字符串。
// 输入必须已经是大写 + '-' 分隔的形态。
func ParseCode(s string) (Code, bool) {
	s = strings.TrimSpace(s)
	if !codeRE.MatchString(s) {
		return "", false
	}
	return Code(s), true
}
