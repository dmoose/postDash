package main

import "testing"

func TestPageRunnerExecuteUsesSlugIntervalCache(t *testing.T) {
	runner := NewPageRunner([]PageConf{
		{
			Name:     "My Page",
			Command:  "printf 'ok'",
			Interval: "1h",
			Format:   "text",
		},
	}, "")

	first := runner.Execute("my-page")
	second := runner.Execute("my-page")
	if first == nil || second == nil {
		t.Fatal("expected non-nil page results")
	}
	if first != second {
		t.Fatal("expected second Execute call to return cached result for slug key")
	}
}
