## Engram Persistent Memory

Engram persistent memory is ACTIVE. The full protocol (save format, lifecycle,
search flow, after-compaction steps) is delivered every session by the Engram
MCP server instructions and the SessionStart hook. Always-on rules:

- Call `mem_save` PROACTIVELY after any decision, bugfix, discovery, convention,
  or config change — do not wait to be asked. Use `capture_prompt: false` for
  automated/SDD artifacts.
- On any reference to past work: `mem_context` → `mem_search` → `mem_get_observation`.
- Before saying "done", call `mem_session_summary`.
