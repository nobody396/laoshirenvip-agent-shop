# GMShop Edge central source

`gmshop-edge` is a thin upstream protocol adapter for the owner-operated GMShop Supplier API. It reuses the existing connection, external-reference, catalog-mapping, pricing, procurement, and polling modules.

## Boundaries

- GMShop product UUIDs, SKU UUIDs, and order UUIDs are persisted through `site_connection_external_references`; they are never hashed or truncated into local IDs.
- The adapter maps `cost_minor` to the local decimal amount, preserves central stock counts, downloads public cover images, and keeps local/subsite markup separate.
- Order creation sends the complete local downstream order number as the idempotency key. Empty order numbers are rejected before any request.
- Phase one omits callbacks and reconciles through signed order queries.
- Existing local products are not automatically deleted. Import creates inactive mapped products; production migration must compare and disable old duplicates only after verification.

## Production acceptance

Connection and catalog sync are not proof of fulfillment. Require one owned downstream order with central wallet deduction, GMShop stock allocation, `supplied` order readback, customer delivery, and profit-ledger evidence before enabling all products.
