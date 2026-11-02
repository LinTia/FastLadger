import 'package:flutter/material.dart';

import 'local_store.dart';

/// 全局应用设置（WBS 5.x 设置页）。
///
/// 状态管理口径：Flutter 内置 [ChangeNotifier]——设置项是典型的"少量全局状态"，
/// 引入 Provider/Riverpod 等第三方库收益不大；页面内部状态仍沿用项目既有
/// 的 StatefulWidget + setState 风格（见 ui/ 各页）。
///
/// 持久化：经 [LocalStore] KV 抽象（骨架期为内存实现，5.2 起接 sqflite/sqlite3）。
/// 通知开关目前是**纯本地偏好**——服务端尚无推送通道（PRD v0.3 规划），
/// 开关只影响客户端内的提醒行为，如实展示不夸大。
class AppSettings extends ChangeNotifier {
  AppSettings({required this.store});

  final LocalStore store;

  static const _kThemeMode = 'fl_theme_mode';
  static const _kNotify = 'fl_notify_enabled';

  ThemeMode _themeMode = ThemeMode.system;
  bool _notifyEnabled = false;
  bool _loaded = false;

  ThemeMode get themeMode => _themeMode;
  bool get notifyEnabled => _notifyEnabled;

  /// 启动时从本地恢复一次（幂等，可重复调用）。
  Future<void> load() async {
    if (_loaded) return;
    _loaded = true;
    final tm = await store.getString(_kThemeMode);
    if (tm == 'light') {
      _themeMode = ThemeMode.light;
    } else if (tm == 'dark') {
      _themeMode = ThemeMode.dark;
    } else {
      _themeMode = ThemeMode.system;
    }
    _notifyEnabled = (await store.getString(_kNotify)) == '1';
    notifyListeners();
  }

  Future<void> setThemeMode(ThemeMode mode) async {
    if (mode == _themeMode) return;
    _themeMode = mode;
    notifyListeners();
    const names = {
      ThemeMode.system: 'system',
      ThemeMode.light: 'light',
      ThemeMode.dark: 'dark',
    };
    await store.setString(_kThemeMode, names[mode]!);
  }

  Future<void> setNotifyEnabled(bool value) async {
    if (value == _notifyEnabled) return;
    _notifyEnabled = value;
    notifyListeners();
    await store.setString(_kNotify, value ? '1' : '0');
  }
}
