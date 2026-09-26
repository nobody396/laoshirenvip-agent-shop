# Agent catalog snapshot

`agent-products.json` is the customer-facing catalog snapshot deployed to the
acceptance database. It records the white-label descriptions, rich product
details, approved storefront asset paths, SKU prices, and SKU display order.

The snapshot intentionally contains no supplier credentials and no upstream
Aisou copy. Database backups remain the recovery source for the live acceptance
environment; this file is the reviewable content baseline for later edits.

## Locale release check

Imported products are not automatically translated. Before publishing a product,
provide `zh-CN`, `zh-TW`, and `en-US` title, description, and content and add the
reviewed copy to this snapshot. Never label copied Chinese text as English.
Run `go test ./catalog` and the read-only live check after publishing:

```sh
python3 scripts/check-catalog-locales.py https://lsrai.shop
python3 scripts/check-catalog-locales.py https://babygptpro.lsrai.shop
```

The 2026-09-26 locale repair covers products 36–38. Their missing English and
traditional-Chinese copy was added; literal escaped newlines were corrected.
The data release changed only localized copy and update timestamps. An exact
pre-change backup, guarded SQL migration, and rollback SQL are retained on the
application server in `backups/locales-20260926/`. Prices, inventory, fulfillment,
and payment configuration were not changed. SMS/iOS SKU labels are localized
at display time, so repeated upstream synchronization does not erase them.
Existing instructional images remain unchanged; embedded image text is not
translated by the storefront language switch.
