package main

import (
	"testing"

	"github.com/John-Robertt/AVMC/internal/domain"
)

func TestFormatFallbackNote(t *testing.T) {
	res := domain.ItemResult{
		ProviderRequested: "javdb",
		ProviderUsed:      "javbus",
		Status:            domain.StatusProcessed,
		Attempts: []domain.ProviderAttempt{
			{Provider: "javdb", Stage: "parse", ErrorCode: domain.ErrCodeParseFailed, ErrorMsg: "返回了验证页"},
			{Provider: "javbus", Stage: "ok"},
		},
	}
	got := formatFallbackNote(res)
	if got == "" {
		t.Fatalf("期望非空 fallback note")
	}
}

func TestFormatAttemptChain(t *testing.T) {
	attempts := []domain.ProviderAttempt{
		{Provider: "javdb", Stage: "fetch", ErrorCode: domain.ErrCodeFetchFailed, ErrorMsg: "HTTP 403"},
		{Provider: "javbus", Stage: "ok"},
	}
	got := formatAttemptChain(attempts, -1)
	if got == "" {
		t.Fatalf("期望非空 attempt chain")
	}
}
