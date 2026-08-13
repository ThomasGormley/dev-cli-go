# dev CLI

`dev` is a personal development CLI that brings selected development workflows behind one command surface.

## Linear integration

**Linear command capability**:
The exact Linear operations needed to carry out one `dev linear` command. Each command names its own capability at its boundary and may compose smaller capabilities, but never uses a shared all-purpose contract.
_Avoid_: shared Linear client interface, `linear.Clienter`

**Linear integration client**:
The application-owned adapter that accesses Linear and satisfies the capabilities required by Linear commands.
_Avoid_: command client

**Linear configuration snapshot**:
The Linear API key, default team selector, and integration client captured for one `dev` invocation and shared by its commands. A command rejects an absent configuration value only when that command requires it.
_Avoid_: per-handler client construction, mutable client factory
