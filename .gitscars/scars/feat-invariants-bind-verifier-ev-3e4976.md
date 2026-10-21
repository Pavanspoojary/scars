---
schema_version: "1"
id: "feat-invariants-bind-verifier-ev-3e4976"
title: "Bind verifier evidence to the recorded command"
slug: "feat-invariants-bind-verifier-ev"
lesson: "The verifier command hash was recorded but never compared against the command it described, so an active scar could claim fail-to-pass verification with a fabricated hash and nothing would object. Bind the recorded evidence to the command it proves, and exclude any scar that violates an invariant from retrieval so that retrieval fails closed instead of injecting an unverified guardrail."
severity: "high"
fix_hint: "Require the recorded command hash to be derived from the recorded argv, and exclude any scar that violates an invariant from retrieval."
good_pattern: "ComputeCommandHash(s.Verifier.Argv, s.Verifier.WorkingDir, s.Verifier.EnvAllowlist)"
applicability: "Recorded evidence of verification, where the evidence must correspond to the command it claims to prove."
base_commit: "8e82bb0fcc944d7a9c5ca62460c3a78dc15c81d9"
fix_commit: "627058f4dbac0c4a4360d6a2d8ccfcc9c4fbbb6f"
anchors:
  files:
    - "pkg/scar/model.go"
    - "pkg/scar/retrieval.go"
  symbols:
    - "ValidateInvariants"
    - "Recall"
verifier:
  result: "fail_to_pass"
  argv:
    - "go"
    - "test"
    - "-race"
    - "./pkg/scar/..."
  command_hash: "b5f170553be68fb27a9807a1f68c3449aa4c0c76362373110011bd73d4978aab"
  output_excerpt: "ok  \\tgithub.com/Pavanspoojary/scars/pkg/scar\\t(cached)\\n"
  output_hash: "fc3cd2f22cc71bfcd2412f28d646f4990bc9eec43378bcdd5378013542068e0b"
baseline:
  recorded_at: "2026-10-21T01:24:39Z"
  command_hash: "b5f170553be68fb27a9807a1f68c3449aa4c0c76362373110011bd73d4978aab"
  pkg/scar/model.go: "aa103fff0fca60cbf4ae888a13fcd3d239d48d797e190342b8d88b4a0b35f911"
  pkg/scar/retrieval.go: "00020477d153a979fe2796e2d910c5a075bc2e0320f4ea89cfb080b16090c25b"
  symbol_hashes:
    recall: "58e72b8ed2cbe940ff2d231bd29ec6ab60c7ea06868569b492883f4f98281a2c"
    validateinvariants: "a4e9db1086c363e304b6a1bf1cca3b292d9d357e489e0df203dc5527af6fe3bd"
review:
  status: "approved"
  reviewer: "maintainer"
  date: "2026-10-21T01:24:54Z"
  rationale: "Reviewed the diff: the recorded command hash was never compared against the command it described, so verification evidence could be asserted independently of the command it claims to prove."
lifecycle:
  state: "active"
---

The verifier command hash was recorded but never compared against the command it described, so an active scar could claim fail-to-pass verification with a fabricated hash and nothing would object. Bind the recorded evidence to the command it proves, and exclude any scar that violates an invariant from retrieval so that retrieval fails closed instead of injecting an unverified guardrail.
