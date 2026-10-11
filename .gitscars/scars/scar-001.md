---
id: scar-001
title: "Avoid unbuffered channel deadlock in worker pool"
created: 2026-10-11T07:11:22Z
status: active
anchors:
  - path: cmd/git-scars/main.go
    symbol: runVerify
    commit: 2c1901f
verifier:
  command: go test ./...
  expected: pass
---

### Symptom & Root Cause
Unbuffered channel causes worker goroutines to block indefinitely under burst load.

### Footgun / Guardrail
Always buffer task channels to worker count or use context cancellation.
