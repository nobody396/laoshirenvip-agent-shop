# Agent shop security audit — 2026-09-22 Beijing

Scope: production image `sha-744f8ad360973a16f7056f61343a278f32f73d96`, database audit and retained application request logs from approximately 2026-09-15 19:00 Beijing. Read-only production investigation; no funds, users, approvals, credentials, or deployment changed.

## Evidence

- 81,468 retained request records in the window; automated reconnaissance includes `/wp-admin/install.php` 1,041 requests, `/.env` 53, `/.git/config` 43, and many PHP/environment filename variants.
- Fresh GETs to `/`, `/.env`, `/.git/config`, `/wp-admin/install.php` all returned identical 472-byte HTML, SHA256 `f1969f3aed47643b5cd6b3332847f227a6365f4e0d606eb80ad9cd5db6d9a8fa`. Current responses are SPA fallback, not demonstrated secret disclosure. Historical bodies were not retained.
- User auth audit: 47 successful and 21 failed logins; admin audit: 37 successful, zero failed. Request logs separately have 4 admin-login and 2 user-login 429s, which do not reach login audit.
- All auth source addresses collapsed to Docker `172.18.0.5` or `172.18.0.1`. Cannot attribute these events to the three VIP source IPs, cannot establish that scans were by the same actor, and cannot use zero IP matches as exoneration.
- Only one active administrator (`owner`), no TOTP enrollment. Recent authorization-change audit contains one reseller approval and its system-domain change, both by administrator 1.
- Email verification configuration is false. All 16 recent users have verification timestamp equal to creation: this is automatic marking, not evidence of an email challenge.
- API credential totals: 5 approved/active, 4 pending/inactive. Mere active status is not an abuse finding.

## Minimal code change, not deployed

Reuse existing Redis-backed middleware to cap aggregate authentication traffic at 30 requests per configured window per source, alongside existing email/source login limits. Shared auth group also covers registration, verification and password recovery. Payment callbacks and upstream/partner order APIs are not inside this group. No new framework, persistence, dependency, or abstraction.

CRITICAL DEPLOYMENT GATE: do not deploy source-global throttling while client addresses collapse to Docker/EdgeOne nodes. First establish and test authenticated/trusted EdgeOne-to-origin client-IP provenance. Gin currently trusts loopback only; Caddy is untrusted Docker peer. Trusting Caddy alone only improves visibility to EdgeOne egress, not real visitor. Do not trust arbitrary client-supplied XFF or EO-Connecting-IP from a directly reachable origin. Pin trusted proxy identity rather than broad trust-all networks.

Tests added: rotating emails across login/registration reach shared cap; another source remains allowed; payment callback remains allowed; router source guard preserves wiring. `gofmt` and `git diff --check` passed. After coordinated disk recovery, `go test -p 1 ./internal/app/httpserver/... -count=1` passed for both router and middleware packages. Real ablation: removing actual auth group middleware caused route guard failure; removing it from the behavioral harness admitted request 31 as 204 instead of expected 429. Restoring both yielded full package PASS. Initial attempts were blocked by disk exhaustion; those failures are superseded only for these two tested packages.

## Outstanding

- Trusted origin/client-IP chain and privacy-safe access logging (do not record credentials, cookies, authorization or sensitive query parameters).
- Controlled deployment and production readback after origin-provenance gate; focused tests and actual ablation are complete.
- Email challenge rollout needs verified business-specific sender and owner-approved recipient tests; auto-marked historical accounts need migration strategy, not mass invalidation.
- Owner enrollment for existing TOTP, backup recovery verification before enforcement.
- Historical per-visitor attribution cannot be reconstructed from collapsed application IP logs alone; EdgeOne retained edge telemetry may narrow it.

### Origin provenance rollout constraints

Production Caddy serves both platform EdgeOne hostnames and direct customer-owned HTTPS domains on ports 80/443. A host-wide EdgeOne-only firewall would break those customer domains: restrict only the `lsrai.shop, *.lsrai.shop` site route. Customer domains must retain direct access without trusting visitor-supplied proxy headers. App is already exposed only at loopback port 8080.

For the platform route: establish official EdgeOne OriginACL ranges and a verified source restriction (prefer independently authenticated origin requests as well), then only consume EdgeOne's overwritten authoritative visitor header from that trusted peer. Strip untrusted forwarding/client-IP headers in the direct-domain route. Pin Caddy's Docker IP in Compose and configure Gin to trust that exact /32; do not widen to all networks. Validate direct forged headers cannot override client identity before deploying the aggregate source limit. Preserve original Caddyfile/Compose and app image for rollback; Caddy has admin API disabled, so plan controlled proxy restart rather than assuming hot reload is available.

## Production completion — Beijing 2026-09-22, approximately 19:32

- EdgeOne OriginACL reports online, version `gaz-0.0.4-20260907`; exact platform/approved platform hostnames only, not redemption/custom domains.
- Caddy platform-host route rejects non-EdgeOne source ranges, then forwards the authoritative EO visitor header as XFF. Direct custom-domain route overwrites XFF with its socket peer and strips EO/X-Real-IP. Gin trusts only fixed proxy `172.29.222.2/32`.
- Dedicated `/29` network contains proxy `.2` and app `.3`. App retains its original default network; DB, Redis and volumes were not recreated. This small network is necessary to avoid reconfiguring the existing live default network while pinning the only trusted proxy. First attempt dynamically allocated app to proxy's reserved address; automatic rollback restored three host health checks; explicitly pinning both peers resolved the collision. No funds or customer identity writes occurred.
- Spoofing XFF, EO-Connecting-IP and X-Real-IP to three TEST-NET values produced real visitor addresses in application audit, not the forged values or Docker addresses.
- Independent gateway-server egress directly connected to origin `187.53.134.185`: main-site `/` and `/health` returned 403; approved custom `ai.dqtx.cc/health` remained 200.
- Runtime image is immutable `ghcr.io/nobody396/laoshirenvip-agent-shop@sha256:891c96caec0ebd621848df8663b3824c8a0a2cd7b8683eba8cddb1591da0c8ed`, OCI revision `d36c6d146aa335b351cbf56c050d1c6d0a6ab10e`. Old production base `744f8ad3` is its ancestor. PR 49 four CI jobs and image run 35720977357 passed.
- Isolated origin-server egress sent 31 empty-JSON registration attempts (no identity, no account created): 30 business-error HTTP 200 responses, then 429. Another egress remained allowed. Under the blocked source: health, homepage and public products 200; unauthenticated supplier API 401; invalid payment callback 404, not auth-throttled. This is boundary verification, not proof of real payment settlement or fulfillment.
- Main site and both approved custom domains remained healthy; `ai`, `acceptance`, `test`, `nova` platform subdomains returned 200 from an independent server. Mac-only TLS failure for `ai` was not reproduced there.
- Existing invitation registration policy unchanged. Email verification remains disabled, owner TOTP unenrolled, and historical automatically marked emails are not genuine email verification.

Rollback artifacts on origin: `Caddyfile.production.pre-security-20260922`, `docker-compose.production.yml.pre-security-20260922`, plus retry backups. Old image tag retained: `sha-744f8ad360973a16f7056f61343a278f32f73d96`. Do not roll back the source-wide limiter alone without preserving provenance; if reverting ingress, first revert the limiter image. No secrets are stored in this report or config backups.
