#!/usr/bin/env python3
"""FastLedger b874146 review counterexamples (stdlib only).

This is NOT a test of a deployed FastLedger backend. The repository has no
buildable backend. It executes verbatim schema/SQL from the Markdown files and
replays explicitly documented algorithms. It does not compile Go/Flutter, run
WebSockets, or execute a formula in Excel. Each result states its evidence type.
Run: python3 reproduce_review.py --repo /path/to/FastLadger
"""
import argparse
import csv
import io
import json
import re
import sqlite3
import tempfile
from pathlib import Path

PIN = "b8741465afa98a89ed99796c9e2c4a841b15a361"
RESULTS = []


def note(name, evidence, actual, expected):
    RESULTS.append(dict(case=name, evidence=evidence, actual=actual, expected=expected))


def block(doc, heading, language):
    part = doc.split(heading, 1)[1]
    return re.search(r"```" + language + r"\n(.*?)\n```", part, re.S).group(1)


def make_db(schema, path=":memory:"):
    db = sqlite3.connect(path, isolation_level=None)
    db.execute("PRAGMA foreign_keys=ON")
    db.executescript(schema)
    return db


def account(db, aid=101, book=1, balance=10000, category=1, version=5):
    db.execute("INSERT INTO accounts(id,book_id,category,name,balance,sync_version) VALUES(?,?,?,?,?,?)",
               (aid, book, category, "synthetic account", balance, version))


def tx(db, tid, book=1, aid=101, amount=1000, category=10, version=0, uuid="", kind=2, when=100):
    db.execute("INSERT INTO transactions(id,book_id,creator_uid,tx_type,account_id,amount,category_id,tx_time,sync_version,client_uuid) VALUES(?,?,?,?,?,?,?,?,?,?)",
               (tid, book, 1, kind, aid, amount, category, when, version, uuid))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo", type=Path, required=True)
    ap.add_argument("--json", type=Path)
    args = ap.parse_args()
    plan = (args.repo / "docs/FastLedger-落地方案.md").read_text(encoding="utf-8")
    guide = (args.repo / "docs/FastLedger-新手开发完整指南.md").read_text(encoding="utf-8")
    schema = block(plan, "## 4.5 完整建表 SQL", "sql")
    tutorial_schema = block(guide, "### 6.4 完整建表 SQL", "sql")
    balance_sql = re.search(r'"(UPDATE accounts SET balance = balance \+ \?.*?)"', plan).group(1)
    version_sql = re.search(r"INSERT INTO book_versions\(book_id, version, updated_at\) VALUES\(\?, 1, \?\)\s+ON CONFLICT\(book_id\) DO UPDATE SET version = version \+ 1, updated_at = excluded.updated_at", plan).group(0)
    budget_sql = re.search(r"`(SELECT COALESCE\(SUM\(amount\),0\) FROM transactions.*?)`", guide, re.S).group(1)

    # Guard the algorithm replays against silently testing an unrelated revision.
    required = ["account.Balance -= tx.Amount", "to.Balance += amount", "totalAssets - totalLiab",
                'Where("client_uuid=?", tx.ClientUuid)', 'default: // 客户端缓冲满，跳过',
                "client.BookIds[b] = true", 'var jwtSecret = []byte("改成你自己的密钥")']
    for anchor in required:
        if anchor not in guide:
            raise SystemExit("Reviewed source changed; re-review required: " + anchor)

    # 1. Same-book membership does not validate the referenced account.
    d = make_db(schema)
    account(d, aid=201, book=2)
    tx(d, 1, book=1, aid=201)
    changed = d.execute(balance_sql, (-1000, 1, 201)).rowcount
    got = d.execute("SELECT book_id,balance FROM accounts WHERE id=201").fetchone()
    assert got == (2, 9000) and changed == 1
    note("cross_book_account", "verbatim DDL + ChangeBalance SQL", dict(account_book=got[0], balance=got[1]), "Reject book 1 write referencing book 2 account; no mutation")
    tx(d, 2, aid=999)
    changed = d.execute(balance_sql, (-1000, 1, 999)).rowcount
    assert changed == 0 and d.execute("SELECT count(*) FROM transactions WHERE id=2").fetchone()[0] == 1
    note("missing_account_zero_affected", "verbatim DDL + ChangeBalance SQL", dict(affected=changed, orphan_transaction=True), "Reject missing/deleted account and roll back transaction")

    # 2. The stated expense / transfer arithmetic, followed by AssetSummary.
    cash, debt = 100000, 0
    debt -= 10000
    net_after_purchase = cash - debt
    assert net_after_purchase == 110000
    cash -= 10000
    debt += 10000
    net_after_repayment = cash - debt
    note("credit_card_sign", "replay of guide 13.1, 13.2 and 16.2 arithmetic", dict(net_after_purchase=net_after_purchase, net_after_repayment=net_after_repayment), dict(net_after_purchase=90000, net_after_repayment=90000))

    # 3. A SQLite statement error need not abort the surrounding transaction.
    d = make_db(schema)
    account(d, aid=101, balance=10000)
    account(d, aid=102, balance=0)
    d.executescript("CREATE TRIGGER injected_failure BEFORE INSERT ON transactions BEGIN SELECT RAISE(ABORT, 'synthetic insert failure'); END;")
    d.execute("BEGIN")
    d.execute("UPDATE accounts SET balance=8000 WHERE id=101")
    d.execute("UPDATE accounts SET balance=2000 WHERE id=102")
    errors = []
    for tid, aid in [(1, 101), (2, 102)]:
        try:
            tx(d, tid, aid=aid, kind=3, amount=2000)
        except sqlite3.DatabaseError as exc:
            errors.append(str(exc))  # same ignored-error control flow as guide 13.2
    d.commit()
    balances = [r[0] for r in d.execute("SELECT balance FROM accounts ORDER BY id")]
    rows = d.execute("SELECT count(*) FROM transactions").fetchone()[0]
    assert balances == [8000, 2000] and rows == 0
    note("transfer_ignored_insert_errors", "SQLite fault injection reproducing guide 13.2 statement/control-flow semantics", dict(commit="succeeded", balances=balances, transaction_rows=rows, ignored_errors=errors), "Return error; balances remain [10000,0]; no partial financial mutation")

    # 4. Both idempotency reads occur before either write transaction starts.
    with tempfile.TemporaryDirectory() as td:
        file = str(Path(td) / "race.db")
        a = make_db(schema, file)
        b = sqlite3.connect(file, isolation_level=None)
        account(a)
        check = "SELECT count(*) FROM transactions WHERE client_uuid=?"
        assert a.execute(check, ("same-request",)).fetchone()[0] == 0
        assert b.execute(check, ("same-request",)).fetchone()[0] == 0
        for db, tid in [(a, 1), (b, 2)]:
            db.execute("BEGIN")
            tx(db, tid, uuid="same-request")
            db.execute(balance_sql, (-1000, 1, 101))
            db.commit()
        count = a.execute(check, ("same-request",)).fetchone()[0]
        balance = a.execute("SELECT balance FROM accounts").fetchone()[0]
        assert count == 2 and balance == 8000
        note("idempotency_check_then_insert", "two SQLite connections; exact pre-check/write ordering", dict(rows=count, balance=balance), dict(rows=1, balance=9000))
        # The precheck is not scoped to a book or user.
        exists = bool(a.execute(check, ("same-request",)).fetchone()[0])
        note("idempotency_scope", "verbatim guide pre-check predicate", dict(book_2_request_would_return_success_without_insert=exists), "Scope operation receipt to authenticated user/book and return original result")
        a.close()
        b.close()

    # 5. Transaction gets a version but its account balance mutation does not.
    d = make_db(schema)
    account(d, version=5)
    d.execute("INSERT INTO book_versions(book_id,version) VALUES(1,5)")
    d.execute("BEGIN")
    d.execute(version_sql, (1, 1))
    v = d.execute("SELECT version FROM book_versions WHERE book_id=1").fetchone()[0]
    tx(d, 1, version=v)
    d.execute(balance_sql, (-1000, 1, 101))
    d.commit()
    rows = d.execute("SELECT id,balance FROM accounts WHERE book_id=1 AND sync_version>5").fetchall()
    assert v == 6 and rows == []
    note("balance_without_sync_version", "verbatim version + balance SQL and documented pull predicate", dict(book_version=v, balance=9000, account_changes_returned=rows), "Changed account is returned with sync_version 6 in the same transaction")

    # 6. The documented lossy WS queue + advancing a global pull cursor.
    queue = list(range(1, 257))
    dropped = 257  # default branch with a full 256-item Send channel
    cursor = max(queue)
    queue.clear()
    cursor = max(cursor, 258)  # later event succeeds and advances lastSyncVersion
    server_versions = [257, 258]
    returned = [v for v in server_versions if v > cursor]
    assert dropped == 257 and returned == []
    note("ws_drop_then_advance", "protocol counterexample; bounded queue/drop and cursor rules from guide 14 + plan 3.5", dict(dropped=dropped, cursor=cursor, pull_returned=returned), "A dropped event triggers catch-up from the last complete pull cursor")

    # 7. Separate entity reads can straddle a committed transaction.
    d = make_db(schema)
    account(d, version=5)
    txs_before = d.execute("SELECT id,sync_version FROM transactions WHERE book_id=1 AND sync_version>5").fetchall()
    d.execute("BEGIN")
    tx(d, 1, version=6)
    d.execute("UPDATE accounts SET balance=9000,sync_version=6 WHERE id=101")
    d.commit()
    accounts_after = d.execute("SELECT id,sync_version FROM accounts WHERE book_id=1 AND sync_version>5").fetchall()
    cursor = max(v for _, v in accounts_after)
    missing = d.execute("SELECT id FROM transactions WHERE book_id=1 AND sync_version>?", (cursor,)).fetchall()
    assert txs_before == [] and cursor == 6 and missing == []
    note("pull_cross_table_snapshot", "SQLite interleaving counterexample to unbounded separate entity reads", dict(first_tx_read=txs_before, later_account_read=accounts_after, next_cursor=cursor, next_pull_transactions=missing), "One consistent read snapshot / safe watermark covers every entity before cursor advances")

    # 8. Restore an older database with a newer client cursor.
    old = make_db(schema)
    account(old)
    old.execute("INSERT INTO book_versions(book_id,version) VALUES(1,5)")
    restored = sqlite3.connect(":memory:", isolation_level=None)
    old.backup(restored)
    restored.execute(version_sql, (1, 2))
    v = restored.execute("SELECT version FROM book_versions WHERE book_id=1").fetchone()[0]
    tx(restored, 1, version=v)
    rows = restored.execute("SELECT id FROM transactions WHERE book_id=1 AND sync_version>10").fetchall()
    assert v == 6 and rows == []
    note("restore_rewinds_sync_counter", "SQLite backup/restore + documented afterVersion query", dict(restored_new_version=v, client_cursor=10, pull_returned=rows), "Restore epoch/reset tells clients to rebuild caches and reconcile offline operations")

    # 9. Parent-category and total budgets do not match the actual SQL predicate.
    d = make_db(schema)
    account(d)
    d.execute("INSERT INTO categories(id,book_id,parent_id,cat_type,name) VALUES(10,1,0,2,'parent'),(11,1,10,2,'child')")
    tx(d, 1, category=11, amount=12000)
    parent = d.execute(budget_sql, (1, 10, 0, 200)).fetchone()[0]
    total = d.execute(budget_sql, (1, 0, 0, 200)).fetchone()[0]
    assert parent == 0 and total == 0
    note("budget_descendants_and_total", "verbatim BudgetProgress SQL", dict(parent_used=parent, total_used=total), dict(parent_used=12000, total_used=12000))

    # 10. SQLite schema divergence produces deterministic missing-column errors.
    one, two = make_db(schema), make_db(tutorial_schema)
    errors = []
    for db, query in [(one, "SELECT password FROM users"), (two, "SELECT members_can_view_others FROM books")]:
        try:
            db.execute(query)
        except sqlite3.OperationalError as exc:
            errors.append(str(exc))
    assert len(errors) == 2
    note("schema_contract_drift", "both verbatim DDL blocks", dict(errors=errors), "One canonical model/DDL naming contract; tutorial SQL and models match the plan")

    # 11. PRAGMA is connection-scoped and there are no declared schema FKs.
    with tempfile.TemporaryDirectory() as td:
        path = str(Path(td) / "pool.db")
        a = make_db(schema, path)
        b = sqlite3.connect(path, isolation_level=None)
        flags = [x.execute("PRAGMA foreign_keys").fetchone()[0] for x in [a, b]]
        fks = a.execute("PRAGMA foreign_key_list(transactions)").fetchall()
        assert flags == [1, 0] and fks == []
        note("pragma_connection_scope", "two real SQLite connections", dict(foreign_keys=flags, declared_transaction_fks=fks), "Set required PRAGMAs on each pooled connection; enforce required references in service/DDL")
        a.close()
        b.close()

    # 12. The OCR rule stops at the comma, before the decimal amount.
    text = "微信支付\n付款成功\n¥ 1,234.56"
    match = re.search(r"[¥￥]\s*([0-9]+\.?[0-9]*)", text)
    cents = round(float(match.group(1)) * 100)
    assert cents == 100
    note("ocr_thousands_separator", "replay of the guide's exact regex on synthetic OCR text", dict(matched=match.group(1), amount_cents=cents), dict(amount_cents=123456))

    # 13. Standard CSV syntax escaping does not make a formula a text cell.
    buf = io.StringIO()
    csv.writer(buf).writerow(["remark", "=1+1"])
    cell = next(csv.reader(io.StringIO(buf.getvalue())))[1]
    assert cell == "=1+1"
    note("csv_formula_unneutralized", "raw CSV field preservation; no Excel execution claimed", dict(exported_text_cell=cell), "Export untrusted text in a spreadsheet-safe text representation; test target spreadsheet import/reopen")

    # 14. Invalid amount/type paths are accepted by the documented arithmetic/schema.
    d = make_db(schema)
    account(d)
    tx(d, 1, amount=-1000)
    d.execute(balance_sql, (1000, 1, 101))
    tx(d, 2, kind=99)
    assert d.execute("SELECT balance FROM accounts").fetchone()[0] == 11000
    note("invalid_transaction_values", "DDL + replay of guide expense/unsupported-type branches", dict(negative_expense_increases_balance_to=11000, unknown_type_inserted=True), "Reject negative/zero/overflow amounts and unsupported tx_type before any write")

    payload = dict(review_commit=PIN, sqlite_runtime=sqlite3.sqlite_version,
                   limitation="Schema/SQL execution and documented-algorithm counterexamples; not Go/Flutter compilation or deployed API security testing.",
                   cases=RESULTS, counterexamples_reproduced=len(RESULTS))
    out = json.dumps(payload, ensure_ascii=False, indent=2)
    if args.json:
        args.json.write_text(out + "\n", encoding="utf-8")
    print(out)


if __name__ == "__main__":
    main()
