import 'package:flutter_test/flutter_test.dart';
import 'package:fastledger_client/parse/receipt_parser.dart';

void main() {
  group('parseReceiptText（与 Go 服务端 ocr_parse 同语义）', () {
    test('微信支出：金额千分位 + 元后缀 + 日期', () {
      final got = parseReceiptText('微信支付\n支出 ¥1,234.56元\n2026-11-01');
      expect(got, hasLength(1));
      expect(got[0].amountCents, 123456);
      expect(got[0].direction, 'expense');
      expect(got[0].occurredDate, '2026-11-01');
      expect(got[0].confidence, 0);
    });

    test('时间 HH:MM 的"时"数字会被视为第二金额 → 整窗丢弃（与服务端保守语义一致）', () {
      // 已知局限（服务端同款）：'12:00' 的 12 命中金额 token，amounts 不唯一 → 丢弃。
      // 客户端与服务端行为必须一致：宁可丢候选，不猜。
      expect(parseReceiptText('支出 ¥1,234.56元 2026-11-01 12:00'), isEmpty);
    });

    test('全角规范化：０-９与全角标点', () {
      final got = parseReceiptText('支出 ￥１２．５０ ２０２６年１１月１日');
      expect(got, hasLength(1));
      expect(got[0].amountCents, 1250);
      expect(got[0].occurredDate, '2026-11-01');
    });

    test('收入方向：到账/收款', () {
      final got = parseReceiptText('收款 ¥100.00 2026/11/01 工资到账');
      expect(got, hasLength(1));
      expect(got[0].direction, 'income');
      expect(got[0].amountCents, 10000);
    });

    test('平台名不代表方向：单独出现不产生候选', () {
      expect(parseReceiptText('微信支付 ¥50.00 2026-11-01'), isEmpty);
      expect(parseReceiptText('支付宝 ¥50.00 2026-11-01'), isEmpty);
    });

    test('平台名 + 收入词正常判定 income', () {
      final got = parseReceiptText('微信支付收入 ¥88.00 2026-11-01');
      expect(got, hasLength(1));
      expect(got[0].direction, 'income');
    });

    test('带符号金额一律拒绝（不取绝对值）', () {
      expect(parseReceiptText('支出 -1.00 2026-11-01'), isEmpty);
      expect(parseReceiptText('支出 +123.00 2026-11-01'), isEmpty);
    });

    test('双方向同时命中 → 丢弃', () {
      expect(parseReceiptText('支出转出又收入到账 ¥50.00 2026-11-01'), isEmpty);
    });

    test('金额不唯一 → 丢弃', () {
      expect(parseReceiptText('支出 ¥10.00 共两笔 ¥20.00 2026-11-01'), isEmpty);
    });

    test('日期不合法 → 丢弃（不猜不补）', () {
      expect(parseReceiptText('支出 ¥10.00 2026-13-40'), isEmpty);
      expect(parseReceiptText('支出 ¥10.00'), isEmpty);
    });

    test('精度超两位 → 丢弃', () {
      expect(parseReceiptText('支出 ¥1.234 2026-11-01'), isEmpty);
    });

    test('千分位分组非法 → 丢弃（服务端同语义）', () {
      expect(parseReceiptText('支出 ¥12,34.5 2026-11-01'), isEmpty);
      expect(parseReceiptText('支出 ¥1,2345.00 2026-11-01'), isEmpty);
    });

    test('OCR 跨行：滑窗合并后可命中', () {
      final got = parseReceiptText('商户：测试超市\n金额\n¥9.90\n2026-11-01 消费');
      expect(got, isNotEmpty);
      expect(got.first.amountCents, 990);
    });

    test('同行去重：同方向同金额同日期只出一个候选', () {
      final got = parseReceiptText('收入 ¥5.00 到账 2026-11-01\n收款 ¥5.00 2026-11-01');
      final keys = got.map((c) => '${c.direction}:${c.amountCents}:${c.occurredDate}').toSet();
      expect(keys.length, got.length);
    });

    test('日期与金额并存时可正确解析', () {
      final got = parseReceiptText('支出 ¥12.00 2026-11-01');
      expect(got, hasLength(1));
      expect(got[0].amountCents, 1200);
    });
  });
}
