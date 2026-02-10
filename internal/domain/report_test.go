package domain

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestRunReport_Finalize_SortAndSummaryAndUTC(t *testing.T) {
	r := RunReport{
		Path:       "/abs/path",
		DryRun:     true,
		StartedAt:  time.Date(2026, 2, 9, 10, 0, 0, 0, time.FixedZone("X", 8*3600)),
		FinishedAt: time.Date(2026, 2, 9, 10, 0, 1, 0, time.FixedZone("X", 8*3600)),
		Items: []ItemResult{
			{Code: "B-02", Status: StatusSkipped},
			{Code: "", Status: StatusFailed}, // config/unmatched 等合成项
			{Code: "A-01", Status: StatusProcessed},
			{Code: "", Status: StatusUnmatched},
		},
	}

	r.Finalize()

	// code=="" 必须排在最后；其内部顺序保持稳定（SliceStable）。
	if r.Items[0].Code != "A-01" || r.Items[1].Code != "B-02" || r.Items[2].Code != "" || r.Items[3].Code != "" {
		t.Fatalf("items 排序不符合契约：%v", []string{r.Items[0].Code, r.Items[1].Code, r.Items[2].Code, r.Items[3].Code})
	}
	if r.Summary.Processed != 1 || r.Summary.Skipped != 1 || r.Summary.Failed != 1 || r.Summary.Unmatched != 1 {
		t.Fatalf("summary 统计不正确：%+v", r.Summary)
	}

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal 失败：%v", err)
	}
	// time.Time 在 UTC 下应输出 'Z' 后缀。
	if len(b) == 0 || !bytes.Contains(b, []byte("\"started_at\":\"2026-02-09T02:00:00Z\"")) {
		t.Fatalf("started_at 不是 UTC RFC3339：%s", string(b))
	}
}
