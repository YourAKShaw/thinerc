# Architecture decision records

Consequential decisions, the alternatives considered, the evidence, and when to revisit them. Each record follows the [Principal Engineer OS ADR template](https://github.com/YourAKShaw/principal-engineer-os/blob/main/assets/adr-template.md). Supersede a decision with a new ADR rather than rewriting history.

| ADR | Decision | Status |
| :--- | :--- | :--- |
| [0001](0001-go-single-static-binary.md) | One static Go binary that emits shell glue | Accepted |
| [0002](0002-static-shell-integration.md) | Static, collision-guarded shell integration with no hooks | Accepted |
| [0003](0003-safety-model.md) | Plan → preview → apply → log → undo for every state change | Accepted |
| [0004](0004-configuration-and-trust.md) | Strict TOML configuration; project files ignored until trusted | Accepted |
| [0005](0005-honest-memory.md) | `freeup` reports available memory and reclaims by stopping real consumers | Accepted |
| [0006](0006-distribution-and-supply-chain.md) | Checksummed, provenance-attested archives; no auto-update, no telemetry | Accepted |
| [0007](0007-agent-integration.md) | Vendor-neutral agent glue; thine never calls a model | Accepted |
| [0008](0008-terminal-ux-and-animation.md) | A Claude-Code-inspired visual language with honest, opt-out animation | Accepted |
