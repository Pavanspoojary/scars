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
  recorded_at: "2026-10-25T20:00:22Z"
  command_hash: "e067222721ad0d3d90c18312f20a51eb36a8c75b859a2d143479d0b39f733a4d"
  pkg/scar/store.go: "b7ea9e46c46e5dc1740739839f48c67e1479fab5f4fb31d2f508045a41663121"
  symbol_hashes:
    ComputeFreshness: "5004afda178a26c9e228ad04e612c3106ca1b1451cc47e856a3430b4ea1f1899"
    ListScars: "358301df16272e204d89f6ad020fa3ff702d387e26ce79f156a64aa1be411b33"
review:
  status: "approved"
  reviewer: "maintainer"
  date: "2026-10-11T08:00:00Z"
  rationale: "Reaffirmed: Retrieval cache stage edited store.go: the catalog worker pool still buffers its job channel to the worker count (make(chan string, len(mdFiles))) and closes it before waiting, so the guarded pattern holds. Verified by go test -race ./... with the new cache under concurrent recall."
lifecycle:
  state: "active"
---

### Symptom & Root Cause
Unbuffered channels cause worker goroutines to block indefinitely when producer dispatch bursts exceed worker consumption capacity.

### Footgun / Guardrail
Always buffer task channels to worker count or use context cancellation with timeout.
