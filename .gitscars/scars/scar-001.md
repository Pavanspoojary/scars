---
schema_version: "1"
id: "scar-001"
title: "Avoid unbuffered channel deadlock in worker pool"
slug: "avoid-unbuffered-channel-deadlock-in-worker-pool"
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
  command_hash: "2d4b1ee683d6be106b1633c27fedb692d3f757a5aa049397e28ef1cbd6d52736"
  cmd/git-scars/main.go: "84376b48e7617c95089212122630f1ed6bf7117802eed4e81f6c366db66c90f8"
  symbol_hashes:
    run: "a04453ed4eb45118c25c3a9ef8db55a5bed059c461a1a7fd4677d04b2eb13ad5"
review:
  status: "approved"
  reviewer: "maintainer"
  date: "2026-10-11T08:00:00Z"
  rationale: "Reaffirmed: Updated anchor baseline after v1.2.0 evolution (harvest, bundle, heal, sarif subcommands added to main.go)"
lifecycle:
  state: "active"
---

### Symptom & Root Cause
Unbuffered channels cause worker goroutines to block indefinitely when producer dispatch bursts exceed worker consumption capacity.

### Footgun / Guardrail
Always buffer task channels to worker count or use context cancellation with timeout.
