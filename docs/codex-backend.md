# Codex Backend Service

`/codex-backend/codex` exposes the Codex client-compatible backend surface. The same handlers are also mounted at `/api/codex`, `/backend-api/codex`, and each prefix with `/v1` appended because Codex client configurations in the upstream Rust workspace use all of these base URL shapes.

## Implemented Endpoints

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/codex-backend/codex/models` | Returns Codex model metadata in the `ModelsClient` response shape. Models from `gpt_codex_models` are returned first by descending priority, then built-in defaults are appended. |
| `POST` | `/codex-backend/codex/responses` | Codex-compatible Responses API entrypoint. Reuses the existing relay path for auth, model distribution, billing, and streaming. |
| `POST` | `/codex-backend/codex/responses/compact` | Codex-compatible compaction entrypoint. Reuses the existing responses compaction relay path. |
| `POST` | `/codex-backend/codex/images/generations` | Codex image generation entrypoint. Reuses the existing image relay path. |
| `POST` | `/codex-backend/codex/images/edits` | Codex image edit entrypoint. Reuses the existing image relay path. |

## Registered Compatibility Prefixes

All implemented endpoints are registered under:

- `/codex-backend/codex`
- `/codex-backend/codex/v1`
- `/api/codex`
- `/api/codex/v1`
- `/backend-api/codex`
- `/backend-api/codex/v1`

## Explicitly Unsupported Endpoints

These endpoints are registered and return `501 Not Implemented` with an OpenAI-style error body until their backing services are added:

| Method | Path | Reason |
| --- | --- | --- |
| `POST` | `/codex-backend/codex/alpha/search` | Requires a Codex encrypted search backend. |
| `POST` | `/codex-backend/codex/memories/trace_summarize` | Requires memory trace summarization semantics. |
| `POST` | `/codex-backend/codex/realtime/calls` | Requires the Codex backend WebRTC call shape. |

## Tables

| Table | Model | Description |
| --- | --- | --- |
| `gpt_codex_models` | `model.CodexBackendModel` | Optional model metadata overrides for the Codex models endpoint. |
