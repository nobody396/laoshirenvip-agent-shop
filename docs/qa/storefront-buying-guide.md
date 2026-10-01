# Storefront buying guide

The homepage shared purchase notice contains a single localized guide and breathing CTA. Classic/list/vault and reseller tenants reuse it. No backend, pricing, payment, delivery, inventory or supplier configuration change.

- Decision tree copied from GMShop bdf1c31, with explicit local product/SKU mappings. Pro $200 current iOS stays iOS; current PH matching PHP 8,919.64 may choose dedicated renewal or acknowledged iOS overwrite. Free can choose PH/iOS. Waiting has no purchase link. Pending upgrade never substitutes another SKU.
- Dedicated PH renewal targets product35/SKU29, not inactive product31/SKU28. Active iOS $200 is product31/SKU4, PH new SKU24, $500 SKU35.
- Prices and stock use the current same-origin product API and existing money/stock helpers, preserving tenant prices. No hardcoded prices, no automatic cart or order creation. Missing/disabled/zero-stock items and unacknowledged overwrite cannot expose a purchase link. Stale requests are discarded.
- `?sku=ID` strictly selects that SKU in both product templates; invalid/removed requests do not select another item. User-explicit selection is preserved on stock refresh. Scroll occurs after product load; no early router hash scroll.
- Examples use the unchanged owner-approved images in nested focus-trapped Reka dialogs. Closing keeps the current question. Reduced-motion CSS and focus/hover/open guards stop the 2.8s breathing animation.
- Vue typecheck + storefront production build passed; 107 Node storefront tests passed; Go web handler tests passed. Initial missing close label and premature hash-scroll warning were found and fixed during browser QA. No real purchase/email/stock mutation during checks.
- Backend source is byte-identical to the current production b78e329 baseline. Latest origin/main changes outside the frontend are confined to the separate invoice email Worker; that Worker is not deployed by this task.
- Reused an idle merged worktree, preserving its node_modules dependency target used by other worktrees. The primary checkout's unrelated ProductDetail price WIP and test remain untouched. Before/after public description backups are under /Users/fujunhao/laoshirenai/tmp/agent-guide-description-*.json; only the obsolete Pro500 pending-channel suffix was removed.
