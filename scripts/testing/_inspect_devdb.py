import sqlite3, os, sys

db_path = r"E:\Users\orang\Downloads\Compressed\hcz_v1\db\hcz.db"
print("DB exists:", os.path.exists(db_path), "size:", os.path.getsize(db_path))

conn = sqlite3.connect(db_path)
conn.row_factory = sqlite3.Row
cur = conn.cursor()

print("\n=== TABLES ===")
cur.execute("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
tables = [r[0] for r in cur.fetchall()]
for t in tables:
    print(" -", t)

def count(sql, args=()):
    try:
        cur.execute(sql, args)
        return cur.fetchone()[0]
    except Exception as e:
        return f"ERR:{e}"

print("\n=== ROW COUNTS (key tables) ===")
key_tables = [
    "c2c_listings", "c2c_trades", "c2c_payment_methods", "c2c_disputes",
    "wallet_accounts", "wallet_transactions", "wallet_recharge_orders",
    "affiliate_commission_ledgers", "affiliate_commissions", "affiliate_profiles",
    "users",
]
for t in key_tables:
    if t in tables:
        print(f" {t:35s} = {count(f'SELECT COUNT(*) FROM {t}')}")
    else:
        print(f" {t:35s} = <MISSING>")

print("\n=== c2c_listings status breakdown ===")
if "c2c_listings" in tables:
    try:
        cur.execute("SELECT status, COUNT(*), SUM(available_usdt), SUM(total_usdt) FROM c2c_listings GROUP BY status")
        for r in cur.fetchall():
            print(" ", dict(r))
    except Exception as e:
        print(" ERR", e)

print("\n=== c2c_trades status breakdown ===")
if "c2c_trades" in tables:
    try:
        cur.execute("SELECT status, COUNT(*) FROM c2c_trades GROUP BY status")
        for r in cur.fetchall():
            print(" ", dict(r))
    except Exception as e:
        print(" ERR", e)

print("\n=== wallet freeze references (c2c_freeze:listing:%) ===")
if "wallet_transactions" in tables:
    try:
        cur.execute("""SELECT reference, user_id, amount, type, direction FROM wallet_transactions
                       WHERE reference LIKE 'c2c_freeze:listing:%' ORDER BY id""")
        rows = cur.fetchall()
        print(" count:", len(rows))
        for r in rows[:20]:
            print(" ", dict(r))
    except Exception as e:
        print(" ERR", e)

print("\n=== wallet_accounts sample ===")
if "wallet_accounts" in tables:
    cur.execute("SELECT user_id, available_balance, frozen_balance FROM wallet_accounts ORDER BY id LIMIT 20")
    for r in cur.fetchall():
        print(" ", dict(r))
    print(" total accounts:", count("SELECT COUNT(*) FROM wallet_accounts"))
    print(" sum available:", count("SELECT COALESCE(SUM(available_balance),0) FROM wallet_accounts"))
    print(" sum frozen:", count("SELECT COALESCE(SUM(frozen_balance),0) FROM wallet_accounts"))

print("\n=== affiliate ledger type breakdown ===")
if "affiliate_commission_ledgers" in tables:
    try:
        cur.execute("SELECT type, COUNT(*), SUM(amount) FROM affiliate_commission_ledgers GROUP BY type")
        for r in cur.fetchall():
            print(" ", dict(r))
    except Exception as e:
        print(" ERR", e)

conn.close()
