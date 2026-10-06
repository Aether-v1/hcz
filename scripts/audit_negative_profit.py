"""
HCZ Go - Read-only Negative Profit Product Audit
Queries the SQLite dev database to identify potentially unprofitable products.
Does NOT modify any data.
"""
import sqlite3
import json
import os

DB_PATH = os.path.join(os.path.dirname(__file__), '..', 'db', 'hcz.db')

def main():
    conn = sqlite3.connect(f'file:{DB_PATH}?mode=ro', uri=True)
    conn.row_factory = sqlite3.Row
    cur = conn.cursor()

    print("=" * 70)
    print("UNPROFITABLE_PRODUCT_AUDIT (READ-ONLY)")
    print("=" * 70)

    # 1. List relevant tables
    cur.execute("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
    tables = [r[0] for r in cur.fetchall()]
    print(f"\n[Tables] Total: {len(tables)}")
    relevant = [t for t in tables if any(k in t.lower() for k in
                ['product', 'order', 'setting', 'exchange', 'wallet', 'affiliate', 'sku'])]
    print(f"[Relevant tables]: {relevant}")

    # 2. Check products table schema
    if 'products' in tables:
        cur.execute("PRAGMA table_info(products)")
        cols = [(r[1], r[2]) for r in cur.fetchall()]
        print(f"\n[products columns]: {cols}")

        cur.execute("SELECT COUNT(*) as cnt FROM products")
        total = cur.fetchone()['cnt']
        cur.execute("SELECT COUNT(*) as cnt FROM products WHERE is_active = 1 AND deleted_at IS NULL")
        active = cur.fetchone()['cnt']
        print(f"[products] Total: {total}, Active: {active}")

        # 3. Audit active products for negative profit
        print("\n" + "-" * 70)
        print("ACTIVE PRODUCT PROFIT AUDIT")
        print("-" * 70)

        cur.execute("""
            SELECT id, slug, price_amount, cost_price_amount, is_active,
                   is_affiliate_enabled, is_mapped
            FROM products
            WHERE is_active = 1 AND deleted_at IS NULL
            ORDER BY id
        """)
        products = [dict(r) for r in cur.fetchall()]

        missing_cost = []
        zero_cost = []
        negative_margin = []
        low_margin = []  # < 10%
        healthy = []

        for p in products:
            price = float(p['price_amount'] or 0)
            cost = float(p['cost_price_amount'] or 0)
            p['price'] = price
            p['cost'] = cost

            if cost == 0:
                zero_cost.append(p)
                continue
            if price <= 0:
                missing_cost.append(p)
                continue

            margin = (price - cost) / price * 100
            p['margin_pct'] = round(margin, 2)
            p['profit_per_unit'] = round(price - cost, 2)

            if price <= cost:
                negative_margin.append(p)
            elif margin < 10:
                low_margin.append(p)
            else:
                healthy.append(p)

        print(f"\n  Total active products: {len(products)}")
        print(f"  Zero cost (cost=0): {len(zero_cost)}")
        print(f"  Negative margin (sale <= cost): {len(negative_margin)}")
        print(f"  Low margin (<10%): {len(low_margin)}")
        print(f"  Healthy (>=10%): {len(healthy)}")

        if negative_margin:
            print("\n  [NEGATIVE MARGIN PRODUCTS]:")
            for p in negative_margin[:20]:
                print(f"    ID={p['id']} slug={p['slug']} price={p['price']} cost={p['cost']} "
                      f"profit={p['profit_per_unit']} margin={p['margin_pct']}% "
                      f"affiliate={p['is_affiliate_enabled']} mapped={p['is_mapped']}")

        if low_margin:
            print("\n  [LOW MARGIN PRODUCTS (<10%)]:")
            for p in low_margin[:20]:
                print(f"    ID={p['id']} slug={p['slug']} price={p['price']} cost={p['cost']} "
                      f"profit={p['profit_per_unit']} margin={p['margin_pct']}% "
                      f"affiliate={p['is_affiliate_enabled']}")

        if zero_cost:
            print("\n  [ZERO COST PRODUCTS (cost_price_amount=0)]:")
            for p in zero_cost[:20]:
                print(f"    ID={p['id']} slug={p['slug']} price={p['price']} "
                      f"affiliate={p['is_affiliate_enabled']} mapped={p['is_mapped']}")

    # 4. Check exchange rate state in settings
    print("\n" + "-" * 70)
    print("EXCHANGE RATE STATE")
    print("-" * 70)
    if 'settings' in tables:
        cur.execute("SELECT key, value FROM settings WHERE key LIKE '%exchange%' OR key LIKE '%rate%'")
        rate_settings = [dict(r) for r in cur.fetchall()]
        if rate_settings:
            for rs in rate_settings:
                print(f"  {rs['key']}: {rs['value'][:200] if rs['value'] else 'NULL'}")
        else:
            print("  No exchange rate settings found (may use separate KV store)")

        # Check all settings keys
        cur.execute("SELECT key FROM settings ORDER BY key")
        all_keys = [r[0] for r in cur.fetchall()]
        print(f"\n  [All settings keys]: {all_keys}")

    # 5. Check orders for rate snapshot
    if 'orders' in tables:
        cur.execute("PRAGMA table_info(orders)")
        order_cols = [r[1] for r in cur.fetchall()]
        rate_fields = [c for c in order_cols if 'rate' in c.lower() or 'usdt' in c.lower() or 'exchange' in c.lower()]
        print(f"\n[orders] Rate/USDT fields: {rate_fields}")

        cur.execute("SELECT COUNT(*) as cnt FROM orders")
        order_count = cur.fetchone()['cnt']
        cur.execute("SELECT COUNT(*) as cnt FROM orders WHERE exchange_rate IS NOT NULL AND exchange_rate > 0")
        orders_with_rate = cur.fetchone()['cnt']
        cur.execute("SELECT COUNT(*) as cnt FROM orders WHERE usdt_total_amount > 0")
        orders_with_usdt = cur.fetchone()['cnt']
        print(f"[orders] Total: {order_count}, With exchange_rate snapshot: {orders_with_rate}, "
              f"With usdt_total_amount: {orders_with_usdt}")

        # Sample recent orders
        cur.execute("""
            SELECT id, order_no, currency, total_amount, usdt_total_amount,
                   exchange_rate, exchange_rate_source, wallet_paid_amount,
                   refunded_amount, status
            FROM orders
            ORDER BY id DESC LIMIT 5
        """)
        recent = [dict(r) for r in cur.fetchall()]
        print("\n  [Recent orders sample]:")
        for o in recent:
            print(f"    #{o['id']} {o['order_no']} status={o['status']} "
                  f"total={o['total_amount']}{o['currency']} usdt={o['usdt_total_amount']} "
                  f"rate={o['exchange_rate']}({o['exchange_rate_source']}) "
                  f"wallet_paid={o['wallet_paid_amount']} refunded={o['refunded_amount']}")

    # 6. SKU cost check
    if 'product_skus' in tables:
        cur.execute("PRAGMA table_info(product_skus)")
        sku_cols = [r[1] for r in cur.fetchall()]
        cost_fields = [c for c in sku_cols if 'cost' in c.lower() or 'price' in c.lower()]
        print(f"\n[product_skus] Cost/Price fields: {cost_fields}")
        cur.execute("SELECT COUNT(*) as cnt FROM product_skus WHERE cost_price_amount > 0")
        skus_with_cost = cur.fetchone()['cnt']
        cur.execute("SELECT COUNT(*) as cnt FROM product_skus")
        total_skus = cur.fetchone()['cnt']
        print(f"[product_skus] Total: {total_skus}, With cost>0: {skus_with_cost}")

    conn.close()
    print("\n" + "=" * 70)
    print("AUDIT COMPLETE (read-only, no data modified)")
    print("=" * 70)

if __name__ == '__main__':
    main()
