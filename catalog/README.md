# Agent catalog snapshot

`agent-products.json` is the customer-facing catalog snapshot deployed to the
acceptance database. It records the rewritten 老实人VIP descriptions, rich
product details, reused GMShop cover paths, SKU prices, and SKU display order.

The snapshot intentionally contains no supplier credentials and no upstream
Aisou copy. Database backups remain the recovery source for the live acceptance
environment; this file is the reviewable content baseline for later edits.
