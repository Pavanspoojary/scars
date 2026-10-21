---
schema_version: "1"
id: "record-the-baseline-capture-time-960ba3"
title: "Record the baseline capture timestamp"
slug: "record-the-baseline-capture-time"
lesson: "Re-recording a baseline refreshed the anchor and symbol hashes but left the capture timestamp untouched, so a reaffirmed baseline continued to report the time of the original capture and understated the age of the evidence. Set the capture timestamp whenever the baseline is re-recorded."
severity: "low"
base_commit: "7b64711f2fa7b9be345e9b09b059cbe011840e85"
fix_commit: "f32cf02df47c2be1a7da52fe77cd6bb92f0eb5e9"
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
  recorded_at: "2026-10-21T01:24:44Z"
  command_hash: "b5f170553be68fb27a9807a1f68c3449aa4c0c76362373110011bd73d4978aab"
  pkg/scar/store.go: "8dad12f8963c3f72ca8ebf59e966bdc08ce842783cb3edcba9015fffc2392980"
  symbol_hashes:
    recordbaseline: "ae0a5bac906ef37fcbde49d7f747801e111df105c330c7c84400e1ab8d785b3e"
review:
  status: "approved"
  reviewer: "maintainer"
  date: "2026-10-21T01:24:54Z"
  rationale: "Reviewed the diff: re-recording a baseline refreshed the hashes but not the capture timestamp, so a reaffirmed baseline reported the age of the original capture."
lifecycle:
  state: "active"
  provenance: "git-commit:f32cf02df47c2be1a7da52fe77cd6bb92f0eb5e9"
---

Re-recording a baseline refreshed the anchor and symbol hashes but left the capture timestamp untouched, so a reaffirmed baseline continued to report the time of the original capture and understated the age of the evidence. Set the capture timestamp whenever the baseline is re-recorded.
