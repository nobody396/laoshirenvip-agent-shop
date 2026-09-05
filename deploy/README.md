# Deployment boundary

This Compose project is the isolated runtime for the agent shop. It binds the
application only to `127.0.0.1:8080`; an HTTPS reverse proxy is added only after
the owned domain and wildcard certificate are verified.

Production secrets are not stored in this repository. Generate and keep them in
Agent Switch, then inject them into the remote runtime without printing them.

Persistent state is split into PostgreSQL, Redis AOF, uploads, and logs. Back up
PostgreSQL and uploads independently; Redis is a queue/cache recovery aid, not
the accounting source of truth.
