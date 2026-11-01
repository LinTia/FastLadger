import 'package:flutter_test/flutter_test.dart';
import 'package:fastledger_client/core/money.dart';

void main() {
  group('parseYuanToCents（字符串整数运算，禁 double）', () {
    test('常规/千分位/整数', () {
      expect(parseYuanToCents('12.34'), 1234);
      expect(parseYuanToCents('1,234.56'), 123456);
      expect(parseYuanToCents('12'), 1200);
      expect(parseYuanToCents('0.5'), 50);
      expect(parseYuanToCents(' 12.30 '), 1230);
    });

    test('非法输入返回 null', () {
      expect(parseYuanToCents(''), isNull);
      expect(parseYuanToCents('-1.00'), isNull); // 负数拒绝
      expect(parseYuanToCents('+3'), isNull);
      expect(parseYuanToCents('1.234'), isNull); // 精度超两位
      expect(parseYuanToCents('1.2.3'), isNull);
      expect(parseYuanToCents('abc'), isNull);
      expect(parseYuanToCents('12a'), isNull);
      expect(parseYuanToCents('0'), isNull); // 零金额无意义
      expect(parseYuanToCents('.5'), isNull); // 缺整数位
    });
  });

  group('centsToYuan', () {
    test('往返一致', () {
      expect(centsToYuan(123456), '1,234.56');
      expect(centsToYuan(5), '0.05');
      expect(centsToYuan(50), '0.50');
      expect(centsToYuan(123456789), '1,234,567.89');
      expect(centsToYuan(-1250), '-12.50');
    });

    test('round trip', () {
      for (final cents in [1, 9, 10, 99, 100, 12345, 99999999]) {
        expect(parseYuanToCents(centsToYuan(cents)), cents);
      }
    });
  });
}
