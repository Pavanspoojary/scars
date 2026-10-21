---
schema_version: "1"
id: "fix-parser-separate-parsing-from-1395eb"
title: "Separate parsing from invariant validation"
slug: "fix-parser-separate-parsing-from"
lesson: "Parsing enforced the lifecycle invariants, so a record that broke one failed to parse. The catalog reader discarded unparseable files, which made every invariant violation invisible to check, lint, and doctor and left their invariant probes as dead code. Keep parsing syntactic and apply the invariants separately, so that violations stay loadable and reportable."
severity: "high"
fix_hint: "Keep parsing syntactic and apply the lifecycle invariants separately, so that records which violate them remain loadable and can be reported."
bad_pattern: "if err := scar.ValidateInvariants(); err != nil {"
good_pattern: "scar.validateSchemaVersion()"
applicability: "Any catalog reader that discards files it cannot parse, when the parser also enforces semantic validity."
base_commit: "fa88baea4fe313da70f85c1b4ead0ffa1164c58e"
fix_commit: "f586ff395f42c83a04ad8a4b1e39f73f0b962f81"
anchors:
  files:
    - "pkg/scar/parser.go"
    - "pkg/scar/model.go"
  symbols:
    - "ParseFrontmatter"
    - "ValidateInvariants"
    - "validateSchemaVersion"
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
  recorded_at: "2026-10-21T01:24:43Z"
  command_hash: "b5f170553be68fb27a9807a1f68c3449aa4c0c76362373110011bd73d4978aab"
  pkg/scar/model.go: "aa103fff0fca60cbf4ae888a13fcd3d239d48d797e190342b8d88b4a0b35f911"
  pkg/scar/parser.go: "63268d39a4d580d79a264d01a6e478ced4cdc012dcd0acdbdfc4c102baf36d9a"
  symbol_hashes:
    parsefrontmatter: "f73abe73c611f8560e58f586e07ee56d07b3e594a0ddf7985288ccff4a774cc8"
    validateinvariants: "a4e9db1086c363e304b6a1bf1cca3b292d9d357e489e0df203dc5527af6fe3bd"
    validateschemaversion: "83a7c5cd46805084fd0ccd0f47f3e5340e94584588fa67911521ebea11d2c755"
review:
  status: "approved"
  reviewer: "maintainer"
  date: "2026-10-21T01:24:54Z"
  rationale: "Reviewed the diff: parsing enforced the lifecycle invariants and the catalog reader discarded unparseable files, so every invariant violation was invisible to check, lint, and doctor."
lifecycle:
  state: "active"
---

Parsing enforced the lifecycle invariants, so a record that broke one failed to parse. The catalog reader discarded unparseable files, which made every invariant violation invisible to check, lint, and doctor and left their invariant probes as dead code. Keep parsing syntactic and apply the invariants separately, so that violations stay loadable and reportable.
