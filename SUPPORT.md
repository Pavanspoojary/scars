# Support

## Where to ask

| Your question or report | Where |
| :--- | :--- |
| A bug in `git-scars` | [Open an issue](https://github.com/Pavanspoojary/scars/issues/new/choose) with the Bug report template |
| A feature idea or architectural suggestion | [Open an issue](https://github.com/Pavanspoojary/scars/issues/new/choose) with the Feature request template |
| An accessibility barrier | [Open an issue](https://github.com/Pavanspoojary/scars/issues/new/choose) with the Accessibility barrier template |
| A question about usage | [Open an issue](https://github.com/Pavanspoojary/scars/issues/new/choose) with the question label |
| A security vulnerability | Report it privately, as described in [SECURITY.md](SECURITY.md). Do not open a public issue. |
| Unacceptable behaviour | Report it privately, as described in [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) |

## Before you report

Most reports are faster to resolve with a little context:

1. Run `git-scars doctor`. It reports the store layout, the invariant state, the anchor freshness, the hook, the trust list, the index cache, and the effective configuration. Include its output.
2. Run `git-scars version` and note the release.
3. Include the exact command, the output you received, and what you expected instead.
4. Remove secrets, tokens, and private paths from anything you paste. `git-scars` redacts its own captured output, but pasted text is not scrubbed.

## Using `git-scars` with an agent

The Model Context Protocol server exposes the same retrieval, injection, and triage behaviour as the command line:

```bash
git-scars mcp
```

Client configuration for Claude Desktop, Cursor, and Antigravity is documented in the README. If an agent surfaces a guardrail that does not apply to your change, that is a defect worth reporting: include the prompt, the tool call, and the scar id.

## Contributing

Bug reports are welcome, and so are fixes. [CONTRIBUTING.md](CONTRIBUTING.md) describes the workflow, the verification gate, and the commit conventions. Every change is expected to keep the project free of third-party dependencies.
