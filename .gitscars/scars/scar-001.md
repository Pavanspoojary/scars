---
schema_version: "1"
id: "scar-001"
slug: "avoid-unbuffered-channel-deadlock-in-worker-pool"
title: "Avoid unbuffered channel deadlock in worker pool"
lesson: "Unbuffered task channels cause worker goroutines to block indefinitely under burst load. Always buffer task channels to worker count or pass context cancellation."
applicability: "Go concurrency pipelines using worker goroutines and unbuffered dispatch channels."
anchors:
  files:
    - "cmd/git-scars/main.go"
  symbols:
    - "run"
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
  command_hash: "2d4b1ee683d6be106b1633c27fedb692d3f757a5aa049397e28ef1cbd6d52736"
baseline:
  recorded_at: "2026-10-11T08:00:00Z"
  cmd/git-scars/main.go: "f13ec806bb6a1a1c136ded3f870f83b4380685124243f1818132a6410f85e69b"
review:
  status: "approved"
  reviewer: "Pavanspoojary"
  date: "2026-10-11T08:00:00Z"
  rationale: "Verified fail-to-pass reproduction with Go race detector"
lifecycle:
  state: "active"
---

### Symptom & Root Cause
Unbuffered channels cause worker goroutines to block indefinitely when producer dispatch bursts exceed worker consumption capacity.

### Footgun / Guardrail
Always buffer task channels to worker count or use context cancellation with timeout.
