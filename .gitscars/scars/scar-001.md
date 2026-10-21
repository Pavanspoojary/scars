---
schema_version: "1"
id: "scar-001"
title: "Avoid unbuffered channel deadlock in worker pool"
slug: "avoid-unbuffered-channel-deadlock-in-worker-pool"
lesson: "Unbuffered task channels cause worker goroutines to block indefinitely under burst load. Always buffer task channels to worker count or pass context cancellation."
severity: "high"
fix_hint: "Size the task channel to the worker count, or pass a cancellable context so blocked workers can exit."
bad_pattern: "make(chan string)"
good_pattern: "jobs := make(chan string, len(mdFiles))"
applicability: "Go concurrency pipelines that dispatch tasks to a fixed pool of worker goroutines."
anchors:
  files:
    - "pkg/scar/store.go"
  symbols:
    - "ListScars"
    - "ComputeFreshness"
  commands:
    - "go test -race ./..."
  error_signatures:
    - "fatal error: all goroutines are asleep - deadlock!"
verifier:
  result: "fail_to_pass"
  argv:
    - "go"
    - "test"
    - "-race"
    - "./..."
  command_hash: "e067222721ad0d3d90c18312f20a51eb36a8c75b859a2d143479d0b39f733a4d"
baseline:
  recorded_at: "2026-10-21T00:53:52Z"
  command_hash: "e067222721ad0d3d90c18312f20a51eb36a8c75b859a2d143479d0b39f733a4d"
  pkg/scar/store.go: "66f1033419560c5f06179d8db5fda201b44279239ac55b205e2054f379f99439"
  symbol_hashes:
    ComputeFreshness: "0a3b0100887805bdd07b8caff3b6848779dc1f553ebeffcfb86cca68bcf0f662"
    ListScars: "358301df16272e204d89f6ad020fa3ff702d387e26ce79f156a64aa1be411b33"
review:
  status: "approved"
  reviewer: "maintainer"
  date: "2026-10-11T08:00:00Z"
  rationale: "Reaffirmed: Re-anchored to pkg/scar/store.go where the worker pool actually lives. The previous anchor pointed at cmd/git-scars/main.go, which contains no channels or goroutines. Added severity, fix hint, and literal bad and good patterns extracted from the anchored implementation."
lifecycle:
  state: "active"
---

### Symptom & Root Cause
Unbuffered channels cause worker goroutines to block indefinitely when producer dispatch bursts exceed worker consumption capacity.

### Footgun / Guardrail
Always buffer task channels to worker count or use context cancellation with timeout.
