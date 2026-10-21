---
schema_version: "1"
id: "treat-an-undeclared-severity-as-517e1a"
title: "Treat an undeclared severity as high severity"
slug: "treat-an-undeclared-severity-as"
lesson: "An active guardrail without a severity field was scored as medium, so it could never reach the default high blocking threshold and silently protected nothing. Treat an undeclared severity as high so that an uncalibrated guardrail still blocks."
severity: "high"
base_commit: "627058f4dbac0c4a4360d6a2d8ccfcc9c4fbbb6f"
fix_commit: "7e43b1602ee2f896bc05aa81165ba577962ab4fd"
anchors:
  files:
    - "pkg/scar/enforce.go"
  symbols:
    - "Enforce"
    - "enforceSeverityLevel"
    - "effectiveSeverity"
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
  recorded_at: "2026-10-21T01:24:48Z"
  command_hash: "b5f170553be68fb27a9807a1f68c3449aa4c0c76362373110011bd73d4978aab"
  pkg/scar/enforce.go: "27264ea5a58f4a58da7e72e70e9a0f6f627d3625a03130a83e4d99561139cfe9"
  symbol_hashes:
    effectiveseverity: "b1b5dfb13ddc8046b299f42324abe763eba7abd3761c8690d2cdec67079c69a0"
    enforce: "37c9f9a22685bb21df2d293695591fd4e3e6d5039a562aa332cb74153393ebd1"
    enforceseveritylevel: "54230b16f38ead5e0a135ac799dd9784acbe151375a85eb9a1497b3b47aebeec"
review:
  status: "approved"
  reviewer: "maintainer"
  date: "2026-10-21T01:24:54Z"
  rationale: "Reviewed the diff: an active guardrail without a severity was scored as medium and could never reach the default high blocking threshold, so it silently protected nothing."
lifecycle:
  state: "active"
  provenance: "git-commit:7e43b1602ee2f896bc05aa81165ba577962ab4fd"
---

An active guardrail without a severity field was scored as medium, so it could never reach the default high blocking threshold and silently protected nothing. Treat an undeclared severity as high so that an uncalibrated guardrail still blocks.
