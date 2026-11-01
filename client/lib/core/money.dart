/// 金额换算（整数分 ↔ 元字符串），全程字符串/整数运算，禁 double（PRD 硬约束）。
library;

/// "12.34" / "1,234.5" / "12" → 分；非法（多小数点、精度超两位、负数、非数字）返回 null。
int? parseYuanToCents(String input) {
  var s = input.trim().replaceAll(',', '');
  if (s.isEmpty) return null;
  final parts = s.split('.');
  if (parts.length > 2) return null;
  final whole = parts[0];
  if (whole.isEmpty || !RegExp(r'^[0-9]+$').hasMatch(whole)) return null;
  String cents;
  if (parts.length == 1) {
    cents = '${whole}00';
  } else {
    final frac = parts[1];
    if (frac.isEmpty || frac.length > 2) return null;
    cents = whole + frac.padRight(2, '0');
  }
  final n = int.tryParse(cents);
  if (n == null || n <= 0) return null;
  return n;
}

/// 分 → 元展示字符串（如 123456 → "1,234.56"）。
String centsToYuan(int cents) {
  final negative = cents < 0;
  var abs = negative ? -cents : cents;
  final whole = abs ~/ 100;
  final frac = (abs % 100).toString().padLeft(2, '0');
  final digits = whole.toString();
  final buf = StringBuffer();
  for (var i = 0; i < digits.length; i++) {
    buf.write(digits[i]);
    final remain = digits.length - 1 - i;
    if (remain > 0 && remain % 3 == 0) buf.write(',');
  }
  return '${negative ? '-' : ''}$buf.$frac';
}
