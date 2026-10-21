---
schema_version: "1"
id: "exit-non-zero-when-verification-122d5f"
title: "Exit non-zero when verification does not pass"
slug: "exit-non-zero-when-verification"
lesson: "A failing verifier was reported as a notice on standard error while the command still exited zero, so a failed verification was indistinguishable from a successful one to continuous integration and to scripted callers. Return an error when the verification outcome is not a pass."
severity: "high"
base_commit: "944853b64b23cc5dd870a4f073a142ad1cd1f6fe"
fix_commit: "60c9f80bb3d645cfdd386ec9d05f67fe7de8041e"
anchors:
  files:
    - "cmd/git-scars/main.go"
  symbols:
    - "runVerify"
verifier:
  result: "fail_to_pass"
  argv:
    - "go"
    - "test"
    - "-race"
    - "./cmd/git-scars/..."
  command_hash: "d08fde911cd01583f1512efc8a5809872046e42a46d597cdf65a44809b443f52"
  output_excerpt: "ok  \\tgithub.com/Pavanspoojary/scars/cmd/git-scars\\t2.295s\\n"
  output_hash: "affa02b571a02486f8773cab58cb12e51fd5c5071aee5a3dd7d5c42849b15bdf"
baseline:
  recorded_at: "2026-10-21T01:24:37Z"
  command_hash: "d08fde911cd01583f1512efc8a5809872046e42a46d597cdf65a44809b443f52"
  cmd/git-scars/main.go: "2f37bb6dacd6dd727855f66419cba8e1e53017383c26ca62c3abc04e052185c1"
  symbol_hashes:
    runverify: "ead848d53b992e406b637517882e2ba9439974aac919cd71646f28e4b179808f"
review:
  status: "approved"
  reviewer: "maintainer"
  date: "2026-10-21T01:24:54Z"
  rationale: "Reviewed the diff: a failing verifier was printed as a notice and the command still exited zero, so a failed verification was indistinguishable from a successful one to continuous integration."
lifecycle:
  state: "active"
  provenance: "git-commit:60c9f80bb3d645cfdd386ec9d05f67fe7de8041e"
---

A failing verifier was reported as a notice on standard error while the command still exited zero, so a failed verification was indistinguishable from a successful one to continuous integration and to scripted callers. Return an error when the verification outcome is not a pass.
