---
schema_version: "1"
id: "fix-cli-overlay-the-fix-commit-t-b781db"
title: "Overlay the fix commit tests during verification"
slug: "fix-cli-overlay-the-fix-commit-t"
lesson: "The verification passed no test files to the fail-to-pass procedure, so the base commit was checked with the tests that already existed at the base. A regression test is normally introduced by the fix commit, so the base ran without it and passed, and every genuine regression was reported as not reproducible. Overlay the test files of the fix commit onto the base worktree."
severity: "high"
fix_hint: "Overlay the test files of the fix commit onto the base worktree before running the verifier there."
bad_pattern: "store.VerifyFailToPass(ctx, s, nil, *interactive)"
good_pattern: "store.VerifyFailToPass(ctx, s, testFiles, *interactive)"
applicability: "Verification that must reproduce a regression at the base commit of a fix."
base_commit: "6de4ee638009a0d9b8530a82d70448a79d105fce"
fix_commit: "0b7abd26982149ded48753121cd2a5b67734a97f"
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
  output_excerpt: "ok  \\tgithub.com/Pavanspoojary/scars/cmd/git-scars\\t(cached)\\n"
  output_hash: "1d1970547ec62264005354d831cdcffb42c10f26d36f0bb8c31021bc856d7f5b"
baseline:
  recorded_at: "2026-10-21T01:24:41Z"
  command_hash: "d08fde911cd01583f1512efc8a5809872046e42a46d597cdf65a44809b443f52"
  cmd/git-scars/main.go: "2f37bb6dacd6dd727855f66419cba8e1e53017383c26ca62c3abc04e052185c1"
  symbol_hashes:
    runverify: "ead848d53b992e406b637517882e2ba9439974aac919cd71646f28e4b179808f"
review:
  status: "approved"
  reviewer: "maintainer"
  date: "2026-10-21T01:24:54Z"
  rationale: "Reviewed the diff and the reproduction: the verification never overlaid the fix commit tests onto the base, so the base ran the pre-existing tests and every genuine regression was reported as not reproducible."
lifecycle:
  state: "active"
---

The verification passed no test files to the fail-to-pass procedure, so the base commit was checked with the tests that already existed at the base. A regression test is normally introduced by the fix commit, so the base ran without it and passed, and every genuine regression was reported as not reproducible. Overlay the test files of the fix commit onto the base worktree.
