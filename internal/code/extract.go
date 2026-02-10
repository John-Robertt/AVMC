package code

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/John-Robertt/AVMC/internal/domain"
)

// 允许的 CODE 变体：字母段 + 分隔符变体 + 数字段。
// 注意：这里要求分隔符至少出现一次，避免把类似 "SAMPLE123" 这种噪音误判成 CODE。
var candidateRE = regexp.MustCompile(`(?i)([a-z]{2,6})[\s._-]+([0-9]{2,5})`)

type UnmatchedError struct {
	// Kind: "no_match" 或 "ambiguous"
	Kind string
	// Candidates 仅在 ambiguous 时返回（已排序，保证稳定）。
	Candidates []domain.Code
}

func (e *UnmatchedError) Error() string {
	switch e.Kind {
	case "no_match":
		return "无法从文件名或父目录解析出 CODE"
	case "ambiguous":
		parts := make([]string, 0, len(e.Candidates))
		for _, c := range e.Candidates {
			parts = append(parts, string(c))
		}
		return "解析到多个不同 CODE（ambiguous）：" + strings.Join(parts, ", ")
	default:
		return "unmatched"
	}
}

// Extract 从 VideoFile 的文件名与父目录名中提取唯一 CODE。
// 若提取失败，返回 *UnmatchedError（no_match / ambiguous）。
func Extract(v domain.VideoFile) (domain.Code, error) {
	m := map[domain.Code]struct{}{}

	addCandidates(m, v.Base)

	parent := filepath.Base(filepath.Dir(v.AbsPath))
	addCandidates(m, parent)

	if len(m) == 0 {
		return "", &UnmatchedError{Kind: "no_match"}
	}
	if len(m) > 1 {
		cands := make([]domain.Code, 0, len(m))
		for c := range m {
			cands = append(cands, c)
		}
		sort.Slice(cands, func(i, j int) bool { return string(cands[i]) < string(cands[j]) })
		return "", &UnmatchedError{Kind: "ambiguous", Candidates: cands}
	}
	for c := range m {
		return c, nil
	}
	return "", &UnmatchedError{Kind: "no_match"}
}

func addCandidates(dst map[domain.Code]struct{}, s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}

	matches := candidateRE.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		prefix := strings.ToUpper(m[1])
		num := m[2]
		if c, ok := domain.ParseCode(prefix + "-" + num); ok {
			dst[c] = struct{}{}
		}
	}
}
