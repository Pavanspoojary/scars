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
  recorded_at: "2026-10-21T00:46:10Z"
  command_hash: "2d4b1ee683d6be106b1633c27fedb692d3f757a5aa049397e28ef1cbd6d52736"
  cmd/git-scars/main.go: "456dccc4c08c01d33da949194406d1ab9cab356fccec87f41b3d132956ba1461"
  symbol_hashes:
    run: "6a352bafa5f18839d63f050081535e21aa48085b1726cafb0a5ce1acfb9c107d"
review:
  status: "approved"
  reviewer: "maintainer"
  date: "2026-10-11T08:00:00Z"
  rationale: "Reaffirmed: Reviewed diff: the only change to the anchored symbol 'run' replaced a version literal with the shared package constant. No concurrency semantics changed."
lifecycle:
  state: "active"
---

### Symptom & Root Cause
Unbuffered channels cause worker goroutines to block indefinitely when producer dispatch bursts exceed worker consumption capacity.

### Footgun / Guardrail
Always buffer task channels to worker count or use context cancellation with timeout.
