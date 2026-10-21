# Accessibility

GitScars aims to be usable by everyone who can run a command line tool, including people who use assistive technology.

## Commitment

- The command line interface is the primary interface, and every command produces plain text that a screen reader can read.
- Machine-readable output is available where a visual layout would otherwise hide information. `git-scars enforce` and `git-scars check` emit SARIF, and most commands accept `--json`.
- Diagnostic output names the failing check and the reason, rather than relying on color or position. `git-scars doctor` reports each probe as text with an explicit status.
- Documentation is Markdown with text alternatives for structure: tables have headers, and commands are shown as copyable text rather than as images.
- The project does not require a graphical interface, a mouse, or a specific editor.

## Supported environments

- Operating systems: Linux, macOS, and Windows, through the Go toolchain.
- Interface: a terminal, a shell, and Git. Any terminal that exposes its text to an assistive technology is supported, including terminals used with a screen reader.
- Editor integration: the Model Context Protocol server (`git-scars mcp`) exposes the same information as structured text to clients such as Claude Desktop, Cursor, and other MCP clients.

## Known limitations

- The command line does not currently offer a screen-reader-specific mode, a reduced-motion mode, or configurable verbosity beyond the existing `--json` output. Color is not required to understand any output.
- `git-scars verify` executes a repository's verifier as a subprocess. The accessibility of that verifier's own output is outside this project's control.
- The web pages and dashboards produced by the optional marketing skills are not part of the git-scars product and are not covered by this statement.
- We have not yet run a formal audit against a specific assistive technology and version. Reports of real barriers are how that gap gets closed.

## Reporting a barrier

If you hit an accessibility barrier, please open an issue using the Accessibility barrier template, or a regular issue labeled `accessibility`. Include:

- The command you ran and the output you received.
- The assistive technology, operating system, and terminal you used.
- What you expected to be able to do, and what stopped you.

Do not include secrets, tokens, or private paths in the report.

Security vulnerabilities are reported separately, through the process in [SECURITY.md](SECURITY.md).
