#!/usr/bin/env python3
"""Read-only release check: a public catalog response must have real English copy.

Usage: python3 scripts/check-catalog-locales.py https://example.com
       python3 scripts/check-catalog-locales.py /path/to/public-products.json
"""
import json
import re
import sys
import urllib.request


def check(products):
    errors = []
    if not products:
        errors.append('catalog is empty')
    for product in products:
        for field in ('title', 'description', 'content'):
            values = product.get(field, {})
            english = values.get('en-US', '')
            if not english.strip() or re.search(r'[\u3400-\u9fff]', english):
                errors.append(f"product {product['id']}: missing/Chinese en-US {field}")
            for locale, value in values.items():
                if re.search(r'\\+n', value):
                    errors.append(f"product {product['id']}: escaped newline in {locale} {field}")
    return errors


if __name__ == '__main__':
    source = sys.argv[1]
    if source.startswith('https://'):
        products = []
        page = 1
        while True:
            url = source.rstrip('/') + f'/api/v1/public/products?page={page}&page_size=100'
            with urllib.request.urlopen(url, timeout=30) as response:
                payload = json.load(response)
            if payload.get('status_code') != 0:
                raise SystemExit('catalog API returned an error')
            products.extend(payload['data'])
            if page >= payload.get('pagination', {}).get('total_page', 1):
                break
            page += 1
    else:
        with open(source) as file:
            payload = json.load(file)
        products = payload['data'] if isinstance(payload, dict) else payload
    errors = check(products)
    if errors:
        raise SystemExit('\n'.join(errors))
    print(f'PASS: {len(products)} products; English copy and newline encoding checked')
