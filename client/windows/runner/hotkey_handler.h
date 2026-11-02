// Windows 平台通道：全局热键 + 前台窗口截图（WBS 5.3）。
//
// 通道 `fastledger/hotkey`，Dart 侧封装见 lib/hotkey/hotkey_service.dart。
// 方法：
//   registerHotkey(modifier:int, vk:int) -> bool   RegisterHotKey 成功与否
//   unregisterHotkey() -> void
//   screenshot() -> Uint8List(PNG)                  GetForegroundWindow + PrintWindow
// 回调：
//   onHotkey（无参）                                 WM_HOTKEY 时 invoke
//
// 注意：本文件在无 Visual Studio 的开发机上无法编译验证（analyze/test 不覆盖
// C++），首次 Windows 构建时须人工复核。实现保持标准 Win32/GDI+ 用法。

#ifndef RUNNER_HOTKEY_HANDLER_H_
#define RUNNER_HOTKEY_HANDLER_H_

#include <windows.h>

#include <flutter/method_channel.h>
#include <flutter/standard_method_codec.h>

#include <cstdint>
#include <memory>
#include <vector>

// 全局通道实例（在 FlutterWindow::OnCreate 中创建）。
extern std::unique_ptr<flutter::MethodChannel<flutter::EncodableValue>>
    g_hotkey_channel;

// 注册/注销与截图实现（供 MessageHandler 调用）。
bool HotkeyRegister(HWND hwnd, int modifier, int vk);
void HotkeyUnregister(HWND hwnd);
// 返回 PNG 字节；失败返回空向量。
std::vector<uint8_t> CaptureForegroundWindowPng();

#endif  // RUNNER_HOTKEY_HANDLER_H_
