---
schema_version: "1"
id: "replace-the-baseline-anchor-set-9a2b1e"
title: "Replace the baseline anchor set when re-recording"
slug: "replace-the-baseline-anchor-set"
lesson: "Re-recording a baseline merged the new anchor hashes into the existing map, so a re-anchored scar stayed coupled to a file it no longer referenced and was reported as stale whenever that unrelated file changed. Replace the recorded anchor set rather than merging into it."
severity: "medium"
base_commit: "7e43b1602ee2f896bc05aa81165ba577962ab4fd"
fix_commit: "92631ef18e84e71012258f8a8789523817f7941f"
anchors:
  files:
    - "pkg/scar/store.go"
  symbols:
    - "RecordBaseline"
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
  recorded_at: "2026-10-21T01:24:46Z"
  command_hash: "b5f170553be68fb27a9807a1f68c3449aa4c0c76362373110011bd73d4978aab"
  pkg/scar/store.go: "8dad12f8963c3f72ca8ebf59e966bdc08ce842783cb3edcba9015fffc2392980"
  symbol_hashes:
    recordbaseline: "ae0a5bac906ef37fcbde49d7f747801e111df105c330c7c84400e1ab8d785b3e"
review:
  status: "approved"
  reviewer: "maintainer"
  date: "2026-10-21T01:24:54Z"
  rationale: "Reviewed the diff: re-recording a baseline merged into the existing anchor map, so a re-anchored scar stayed coupled to a file it no longer referenced."
lifecycle:
  state: "active"
  provenance: "git-commit:92631ef18e84e71012258f8a8789523817f7941f"
---

Re-recording a baseline merged the new anchor hashes into the existing map, so a re-anchored scar stayed coupled to a file it no longer referenced and was reported as stale whenever that unrelated file changed. Replace the recorded anchor set rather than merging into it.
