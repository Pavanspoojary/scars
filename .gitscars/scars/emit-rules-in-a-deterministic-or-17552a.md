---
schema_version: "1"
id: "emit-rules-in-a-deterministic-or-17552a"
title: "Emit rules in a deterministic order"
slug: "emit-rules-in-a-deterministic-or"
lesson: "SARIF rules were assembled by iterating a map, so two reports of identical findings were ordered differently and produced spurious differences in continuous integration logs and uploaded artefacts. Emit rules through a sorted accessor so that reports of the same findings are byte identical."
severity: "medium"
base_commit: "1a47bc74046651d28378046f40884f5e80cd99d4"
fix_commit: "1923080a5a46ba82b1785f1d3bdefd360c0b6213"
anchors:
  files:
    - "pkg/scar/sarif.go"
  symbols:
    - "FormatEnforceSARIF"
    - "FormatCheckSARIF"
    - "sortedRules"
verifier:
  result: "fail_to_pass"
  argv:
    - "go"
    - "test"
    - "-race"
    - "./pkg/scar/..."
  command_hash: "b5f170553be68fb27a9807a1f68c3449aa4c0c76362373110011bd73d4978aab"
  output_excerpt: "ok  \\tgithub.com/Pavanspoojary/scars/pkg/scar\\t2.155s\\n"
  output_hash: "9f8119fb4579c773e2b9b5ad916bc04c4e16b22bdef63e73a27a85afff624482"
baseline:
  recorded_at: "2026-10-21T01:24:34Z"
  command_hash: "b5f170553be68fb27a9807a1f68c3449aa4c0c76362373110011bd73d4978aab"
  pkg/scar/sarif.go: "bdbc11f7dbb36c053f64945f84feb4a58e5a7fa0b66f09fa5f0fb2bf7b7e48c6"
  symbol_hashes:
    formatchecksarif: "545c2f56260e517f52569958b72a2889bc986b78720c875d60f8b8c9567366c6"
    formatenforcesarif: "8a132b2252e9d78dfde83675ddddd420f9638af6543634352554d22a8f4bd2be"
    sortedrules: "8026b5df88143c1241e2a49f4501b5438c272997d0b8a7501c432d79ccdf5c34"
review:
  status: "approved"
  reviewer: "maintainer"
  date: "2026-10-21T01:24:54Z"
  rationale: "Reviewed the diff and the reproduction: the report assembled its rules by iterating a map, so identical findings produced differently ordered reports. The fix routes rule emission through a sorted accessor."
lifecycle:
  state: "active"
  provenance: "git-commit:1923080a5a46ba82b1785f1d3bdefd360c0b6213"
---

SARIF rules were assembled by iterating a map, so two reports of identical findings were ordered differently and produced spurious differences in continuous integration logs and uploaded artefacts. Emit rules through a sorted accessor so that reports of the same findings are byte identical.
