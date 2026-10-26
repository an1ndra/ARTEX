# LLM providers

ARTEX talks to any OpenAI- or Anthropic-shaped endpoint. Configure in the UI at
**System → LLM Configuration** (multiple profiles, one starred *Active*, each agent can pin
its own), or — for a single provider — through environment variables, which take effect on the
next restart.

Profile formats: `Anthropic`, `OpenAI (Chat Completions)`, `OpenAI (Responses API)`,
`Ollama` (keyless). `Test connection` runs a real one-shot request with the profile's own
streaming/thinking/session settings, so a passing test means conversations will work too.

## Environment variables (single provider)

| Variable | Meaning |
|---|---|
| `ARTEX_LLM_PROVIDER` | `anthropic` \| `openai` \| `openai-responses` \| `ollama` |
| `ARTEX_LLM_MODEL` | model id |
| `ARTEX_LLM_BASE_URL` | endpoint base (no `/chat/completions` suffix needed) |
| `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` | credentials |
| `ARTEX_LLM_PROXY` | optional HTTP/SOCKS5 proxy |
| `ARTEX_LLM_STREAM` | `false` disables streaming |
| `ARTEX_LLM_USER_AGENT` | overrides the `artex/<version>` User-Agent sent to the gateway |

UI profiles override the env config once one is marked active.

## OpenCode Go / OpenCode Console

Both share one API key from `https://opencode.ai/console`; the path segment picks the billing
source: `/go/` bills the Go subscription, no `/go/` bills pay-as-you-go credits.

| Chat family | Format | Base URL |
|---|---|---|
| `deepseek-*`, `kimi-*`, `glm-*`, `minimax-m3`, `mimo-*`, `longcat-*`, `hy*`, `step-*` | OpenAI (Chat Completions) | `https://opencode.ai/inference/go/openai/v1` |
| `claude-*`, `qwen3.7-plus`, `qwen3.8-*` | Anthropic | `https://opencode.ai/inference/go/anthropic` |
| `gpt-*`, `grok-*`, `muse-spark-*` | OpenAI (Responses API) | `https://opencode.ai/inference/go/openai/v1` |

Drop the `/go` segment for the pay-as-you-go endpoints.

Requirements worth respecting:

* **Session header** — set the profile's *Session header* field to `x-opencode-session`.
  Without it Go answers `400 MissingSessionID`. ARTEX sends the real per-session id
  (`conv-<id>`, `exp<n>-worker-i<intent>`), which is what their router and prompt cache key on.
* **User-Agent** — OpenCode asks clients to identify themselves rather than look like a bare HTTP
  library. Since this fork, requests carry `artex/<version>` (override: `ARTEX_LLM_USER_AGENT`);
  before that they went out as Go's default `Go-http-client/1.1`, which their abuse monitoring
  flags. Covered by `agent/provider_useragent_test.go`.
* **Model list** — `https://opencode.ai/inference/go/v1/models` is public and returns the catalog,
  but the UI's *load available models* button probes `<base>/models`
  (`.../go/openai/v1/models`), which is `404`. Type model ids manually.
* Free-tier models (`*-free`) are reserved for the OpenCode client; from ARTEX use paid/Go models.

## Hetzner Inference (experimental)

| Field | Value |
|---|---|
| Format | OpenAI (Chat Completions) |
| Base URL | `https://inference.hetzner.com/api/v1` |
| Models | `Qwen/Qwen3.6-35B-A3B-FP8`, `Qwen3.8-27B` |
| Token | `experiments.hetzner.com` → Inference |

Caveats that decide whether it is usable: **no tool calling** is declared, so it cannot drive
ARTEX's planner/worker agents (which pass a tool set on every turn); the limits are 10 requests
and 4M input tokens per 60 s per key, which a planner + 3 workers exhaust immediately; and
reasoning is on by default, switched by `chat_template_kwargs.enable_thinking`, a field ARTEX
does not send — set the profile's *Thinking switch* to `Disabled` and keep the output cap
generous.
