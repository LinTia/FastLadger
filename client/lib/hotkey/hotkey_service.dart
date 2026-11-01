
import 'package:flutter/services.dart';

/// Windows 平台热键 + 截图服务（WBS 5.3）。
///
/// 平台通道 `fastledger/hotkey`，Windows runner 侧（C++）实现：
/// - `registerHotkey`：注册全局热键（默认 Ctrl+Alt+F），按键后回调 [onHotkey]；
/// - `screenshot`：GDI PrintWindow 截取当前前台窗口 → PNG 字节；
/// - `unregisterHotkey`：退出前注销。
///
/// Dart 侧只依赖抽象通道，widget/单元测试用 TestDefaultBinaryMessenger
/// mock method call 即可，不依赖真实 Windows 环境。
class HotkeyService {
  /// 通道名公开：测试用 TestDefaultBinaryMessenger mock。
  static const MethodChannel channel = MethodChannel('fastledger/hotkey');
  static const MethodChannel _channel = channel;

  final void Function()? onHotkey;
  HotkeyService({required this.onHotkey}) {
    _channel.setMethodCallHandler((call) async {
      if (call.method == 'onHotkey') onHotkey?.call();
      return null;
    });
  }

  /// 注册全局热键。modifier/vk 为 Win32 常量（如 MOD_CONTROL|MOD_ALT=2|1=3，
  /// 'F' 的 VK=0x46）。返回 false = 注册失败（被占用/平台不可用/测试环境）。
  Future<bool> register({int modifier = 3, int vk = 0x46}) async {
    try {
      return await _channel
              .invokeMethod<bool>('registerHotkey', {
            'modifier': modifier,
            'vk': vk,
          }) ??
          false;
    } catch (_) {
      // PlatformException（热键被占用）/MissingPluginException（非 Windows 运行时）
      return false;
    }
  }

  Future<void> unregister() async {
    try {
      await _channel.invokeMethod<void>('unregisterHotkey');
    } catch (_) {
      // 注销失败不阻断退出
    }
  }

  /// 截取前台窗口截图（PNG）。失败抛 PlatformException。
  Future<Uint8List> screenshot() async {
    final data = await _channel.invokeMethod<Uint8List>('screenshot');
    if (data == null || data.isEmpty) {
      throw PlatformException(code: 'screenshot_failed', message: '截图失败');
    }
    return data;
  }
}
