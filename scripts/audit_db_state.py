import sqlite3, json
conn = sqlite3.connect('file:./db/hcz.db?mode=ro', uri=True)
cur = conn.cursor()
print("=== SETTINGS KEYS ===")
cur.execute("SELECT key FROM settings ORDER BY key")
for r in cur.fetchall():
    print(r[0])
print("\n=== EXCHANGE/RATE/CURRENCY SETTINGS ===")
cur.execute("SELECT key, value_json FROM settings WHERE key LIKE '%exchange%' OR key LIKE '%rate%' OR key LIKE '%currency%' OR key LIKE '%site_config%' OR key LIKE '%order_config%'")
for r in cur.fetchall():
    v = r[1]
    if isinstance(v, bytes): v = v.decode('utf-8', errors='replace')
    print(f"{r[0]}: {v[:300] if v else None}")
print("\n=== ORDERS SUMMARY ===")
cur.execute("SELECT COUNT(*) FROM orders")
print(f"Total orders: {cur.fetchone()[0]}")
cur.execute("SELECT COUNT(*) FROM orders WHERE exchange_rate IS NOT NULL AND exchange_rate > 0")
print(f"Orders with rate snapshot: {cur.fetchone()[0]}")
cur.execute("SELECT COUNT(*) FROM orders WHERE usdt_total_amount > 0")
print(f"Orders with usdt_total: {cur.fetchone()[0]}")
cur.execute("SELECT id, order_no, status, currency, total_amount, usdt_total_amount, exchange_rate, exchange_rate_source, wallet_paid_amount, refunded_amount FROM orders ORDER BY id DESC LIMIT 5")
print("\nRecent orders:")
for r in cur.fetchall():
    print(f"  #{r[0]} {r[1]} status={r[2]} cur={r[3]} total={r[4]} usdt={r[5]} rate={r[6]}({r[7]}) wallet_paid={r[8]} refunded={r[9]}")
print("\n=== PRODUCT SKUS ===")
cur.execute("SELECT id, product_id, sku_code, price_amount, cost_price_amount, is_active FROM product_skus ORDER BY id")
for r in cur.fetchall():
    print(f"  sku#{r[0]} product={r[1]} code={r[2]} price={r[3]} cost={r[4]} active={r[5]}")
conn.close()
