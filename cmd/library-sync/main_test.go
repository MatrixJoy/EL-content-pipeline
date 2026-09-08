package main

import (
	"testing"
	"time"
)

func TestNextCrawlDelayUsesBatchDelayOnlyWhenBatchWasFull(t *testing.T) {
	if got := nextCrawlDelay(5000, 5000, false, time.Minute, 6*time.Hour); got != time.Minute {
		t.Fatalf("full batch delay = %v", got)
	}
	if got := nextCrawlDelay(120, 5000, false, time.Minute, 6*time.Hour); got != 6*time.Hour {
		t.Fatalf("idle delay = %v", got)
	}
	if got := nextCrawlDelay(0, 0, false, time.Minute, 6*time.Hour); got != 6*time.Hour {
		t.Fatalf("unlimited run delay = %v", got)
	}
	if got := nextCrawlDelay(0, 5000, true, time.Minute, 6*time.Hour); got != time.Minute {
		t.Fatalf("failure delay = %v", got)
	}
}
