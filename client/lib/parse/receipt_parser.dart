/// 端侧规则引擎（WBS 5.6）：微信/支付宝账单文本 → 交易候选。
///
/// 与服务端 `server/internal/service/ocr_parse.go` 保持同语义（保守契约，不猜）：
/// - 全角规范化后解析；金额支持千分位与"元/人民币"后缀，整数转分（字符串拼接，禁 double）；
/// - 带符号金额一律拒绝（不取绝对值）；
/// - 方向关键词：收入侧 收入/入账/收款/到账/转入，支出侧 支出/付款/消费/转出/扣款；
///   平台名（微信支付/支付宝）不代表收支方向；
/// - 仅当金额、方向、日期三者均可唯一确认时产出候选，歧义 → 空候选；
/// - confidence 恒为 0（规则引擎无置信度模型，0 = 无置信度而非低可信）。
library;

/// 与服务端 OCRCandidate 同构。
class ReceiptCandidate {
  final int amountCents;
  final String direction; // income | expense
  final String occurredDate; // YYYY-MM-DD 或 ''
  final String description;
  final double confidence;
  ReceiptCandidate({
    required this.amountCents,
    required this.direction,
    required this.occurredDate,
    required this.description,
    this.confidence = 0,
  });

  @override
  bool operator ==(Object other) =>
      other is ReceiptCandidate &&
      other.amountCents == amountCents &&
      other.direction == direction &&
      other.occurredDate == occurredDate &&
      other.description == description;

  @override
  int get hashCode =>
      Object.hash(amountCents, direction, occurredDate, description);
}

final RegExp _dateSlash = RegExp(r'(?:^|[^0-9])([0-9]{4})[-/]([0-9]{1,2})[-/]([0-9]{1,2})(?:$|[^0-9])');
final RegExp _dateCn = RegExp(r'(?:^|[^0-9])([0-9]{4})年([0-9]{1,2})月([0-9]{1,2})日?(?:$|[^0-9])');
final RegExp _dateCompact = RegExp(r'(?:^|[^0-9])([0-9]{4})([0-9]{2})([0-9]{2})(?:$|[^0-9])');
final List<RegExp> _datePatterns = [_dateSlash, _dateCn, _dateCompact];

final RegExp _amountToken = RegExp(r'[¥￥$]?\s*[0-9][0-9,]*(?:\.[0-9]+)?\s*(?:元|人民币)?');

const _incomeWords = ['收入', '入账', '收款', '到账', '转入'];
const _expenseWords = ['支出', '付款', '消费', '转出', '扣款'];

/// 主入口：文本 → 候选列表（可能为空）。
List<ReceiptCandidate> parseReceiptText(String text) {
  final lines = normalizeReceipt(text)
      .split(RegExp(r'[\n\r;]+'))
      .where((s) => s.isNotEmpty)
      .toList();
  final candidates = <ReceiptCandidate>[];
  final seen = <String>{};
  for (var i = 0; i < lines.length; i++) {
    final start = i - 1 < 0 ? 0 : i - 1;
    final end = i + 1 >= lines.length ? lines.length - 1 : i + 1;
    final window = lines.sublist(start, end + 1).join(' ').trim();
    if (window.length > 1000) continue; // Dart String.length 对 BMP 中文即字符数
    final income = _hasAny(window, _incomeWords);
    final expense = _hasAny(window, _expenseWords);
    if (income == expense) continue;
    final dates = _extractDates(window);
    if (dates.length != 1) continue;
    final amounts = _extractAmounts(_withoutDates(window));
    if (amounts.length != 1) continue;
    final candidate = ReceiptCandidate(
      amountCents: amounts[0],
      direction: income ? 'income' : 'expense',
      occurredDate: dates[0],
      description: window,
    );
    final key = '${candidate.direction}:${candidate.amountCents}:${candidate.occurredDate}';
    if (seen.contains(key)) continue;
    seen.add(key);
    candidates.add(candidate);
  }
  return candidates;
}

/// 全角规范化（与服务端 normalizeOCR 同表）。
String normalizeReceipt(String s) {
  final b = StringBuffer();
  for (final r in s.runes) {
    var c = r;
    if (c >= 0xFF10 && c <= 0xFF19) {
      c = 0x30 + (c - 0xFF10); // ０-９ → 0-9
    } else if (c == 0xFF0C) {
      c = 0x2C; // ，
    } else if (c == 0xFF0E) {
      c = 0x2E; // ．
    } else if (c == 0xFF1A) {
      c = 0x3A; // ：
    } else if (c == 0xFF0D) {
      c = 0x2D; // －
    }
    b.writeCharCode(c);
  }
  return b.toString();
}

bool _hasAny(String s, List<String> words) => words.any(s.contains);

/// 提取金额（分）。遇到带符号/千分位非法/精度超两位等一律返回空表（整窗放弃）。
List<int> _extractAmounts(String s) {
  final amounts = <int>[];
  for (final m in _amountToken.allMatches(s)) {
    if (m.start > 0) {
      final prev = s.codeUnitAt(m.start - 1);
      if (prev == 0x2D || prev == 0x2B) return const []; // '-' 或 '+'
    }
    var value = m.group(0)!.trim();
    if (value.isEmpty) continue;
    value = value.replaceAll(RegExp(r'^[¥￥$]\s*'), '').trim();
    value = value.replaceAll(RegExp(r'(人民币|元)\s*$'), '').trim();
    if (value.isEmpty || RegExp(r'[-+eE]').hasMatch(value)) return const [];
    if (!_validGroupedAmount(value)) return const [];
    value = value.replaceAll(',', '');
    final parts = value.split('.');
    if (parts.length > 2 ||
        (parts.length == 2 && (parts[1].isEmpty || parts[1].length > 2))) {
      return const [];
    }
    var cents = '${parts[0]}00';
    if (parts.length == 2) {
      cents = parts[0] + parts[1];
      if (parts[1].length == 1) cents += '0';
    }
    final n = int.tryParse(cents);
    if (n == null || n <= 0) return const [];
    amounts.add(n);
  }
  return amounts;
}

bool _validGroupedAmount(String value) {
  final whole = value.split('.')[0];
  if (whole.isEmpty) return false;
  if (whole.contains(',')) {
    final groups = whole.split(',');
    if (groups[0].isEmpty || groups[0].length > 3) return false;
    for (final g in groups) {
      if (g.isEmpty) return false;
      if (!RegExp(r'^[0-9]+$').hasMatch(g)) return false;
    }
    for (final g in groups.sublist(1)) {
      if (g.length != 3) return false;
    }
  } else if (!RegExp(r'^[0-9]+$').hasMatch(whole)) {
    return false;
  }
  return true;
}

String _withoutDates(String s) {
  for (final p in _datePatterns) {
    s = s.replaceAllMapped(p, (_) => ' ');
  }
  return s;
}

/// 提取日期；遇到非法日期（含越界年）返回空表 → 整窗放弃（与服务端同语义）。
List<String> _extractDates(String s) {
  final dates = <String>[];
  for (final p in _datePatterns) {
    for (final m in p.allMatches(s)) {
      final y = int.tryParse(m.group(1)!);
      final mo = int.tryParse(m.group(2)!);
      final d = int.tryParse(m.group(3)!);
      if (y == null || mo == null || d == null ||
          y < 2000 || y > 2100 || !_validDate(y, mo, d)) {
        return const [];
      }
      dates.add('${y.toString().padLeft(4, '0')}-'
          '${mo.toString().padLeft(2, '0')}-${d.toString().padLeft(2, '0')}');
    }
  }
  return dates;
}

bool _validDate(int y, int m, int d) {
  if (m < 1 || m > 12 || d < 1) return false;
  final date = DateTime.utc(y, m, d);
  return date.day == d && date.month == m;
}
