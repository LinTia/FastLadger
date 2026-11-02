import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:fastledger_client/core/app_settings.dart';
import 'package:fastledger_client/core/local_store.dart';

void main() {
  group('AppSettings（ChangeNotifier + LocalStore 持久化）', () {
    test('默认值：跟随系统主题 / 通知关', () {
      final s = AppSettings(store: InMemoryLocalStore());
      expect(s.themeMode, ThemeMode.system);
      expect(s.notifyEnabled, isFalse);
    });

    test('setThemeMode 更新并持久化，load() 恢复', () async {
      final store = InMemoryLocalStore();
      final s = AppSettings(store: store);
      var notified = 0;
      s.addListener(() => notified++);
      await s.setThemeMode(ThemeMode.dark);
      expect(s.themeMode, ThemeMode.dark);
      expect(notified, 1);
      expect(await store.getString('fl_theme_mode'), 'dark');

      // 新实例恢复
      final s2 = AppSettings(store: store);
      await s2.load();
      expect(s2.themeMode, ThemeMode.dark);
    });

    test('setNotifyEnabled 更新并持久化', () async {
      final store = InMemoryLocalStore();
      final s = AppSettings(store: store);
      await s.setNotifyEnabled(true);
      expect(s.notifyEnabled, isTrue);
      expect(await store.getString('fl_notify_enabled'), '1');

      final s2 = AppSettings(store: store);
      await s2.load();
      expect(s2.notifyEnabled, isTrue);
    });

    test('重复设置同值不重复通知', () async {
      final s = AppSettings(store: InMemoryLocalStore());
      var notified = 0;
      s.addListener(() => notified++);
      await s.setThemeMode(ThemeMode.light);
      await s.setThemeMode(ThemeMode.light);
      expect(notified, 1);
    });
  });
}
