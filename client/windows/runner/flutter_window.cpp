#include "flutter_window.h"

#include <flutter/method_channel.h>
#include <flutter/standard_method_codec.h>

#include <memory>
#include <optional>

#include "flutter/generated_plugin_registrant.h"
#include "hotkey_handler.h"

// 与 hotkey_handler.cpp 的 kHotkeyId 一致。
namespace {
constexpr int kFastLedgerHotkeyId = 1;
}

FlutterWindow::FlutterWindow(const flutter::DartProject& project)
    : project_(project) {}

FlutterWindow::~FlutterWindow() {}

bool FlutterWindow::OnCreate() {
  if (!Win32Window::OnCreate()) {
    return false;
  }

  RECT frame = GetClientArea();

  // The size here must match the window dimensions to avoid unnecessary surface
  // creation / destruction in the startup path.
  flutter_controller_ = std::make_unique<flutter::FlutterViewController>(
      frame.right - frame.left, frame.bottom - frame.top, project_);
  // Ensure that basic setup of the controller was successful.
  if (!flutter_controller_->engine() || !flutter_controller_->view()) {
    return false;
  }
  RegisterPlugins(flutter_controller_->engine());
  SetChildContent(flutter_controller_->view()->GetNativeWindow());

  // WBS 5.3：热键/截图平台通道（Dart 侧 lib/hotkey/hotkey_service.dart）。
  const HWND main_hwnd = GetHandle();
  g_hotkey_channel =
      std::make_unique<flutter::MethodChannel<flutter::EncodableValue>>(
          flutter_controller_->engine()->messenger(), "fastledger/hotkey",
          &flutter::StandardMethodCodec::GetInstance());
  g_hotkey_channel->SetMethodCallHandler(
      [main_hwnd](const auto& call, auto result) {
        if (call.method_name() == "registerHotkey") {
          int modifier = 3, vk = 0x46;
          if (const auto* args = std::get_if<flutter::EncodableMap>(
                  call.arguments())) {
            if (auto m = args->find(flutter::EncodableValue("modifier"));
                m != args->end()) {
              modifier = std::get<int>(m->second);
            }
            if (auto v = args->find(flutter::EncodableValue("vk"));
                v != args->end()) {
              vk = std::get<int>(v->second);
            }
          }
          result->Success(
              flutter::EncodableValue(HotkeyRegister(main_hwnd, modifier, vk)));
        } else if (call.method_name() == "unregisterHotkey") {
          HotkeyUnregister(main_hwnd);
          result->Success();
        } else if (call.method_name() == "screenshot") {
          auto png = CaptureForegroundWindowPng();
          if (png.empty()) {
            result->Error("screenshot_failed", "screenshot failed");
          } else {
            result->Success(flutter::EncodableValue(png));
          }
        } else {
          result->NotImplemented();
        }
      });

  flutter_controller_->engine()->SetNextFrameCallback([&]() {
    this->Show();
  });

  // Flutter can complete the first frame before the "show window" callback is
  // registered. The following call ensures a frame is pending to ensure the
  // window is shown. It is a no-op if the first frame hasn't completed yet.
  flutter_controller_->ForceRedraw();

  return true;
}

void FlutterWindow::OnDestroy() {
  HotkeyUnregister(GetHandle());
  g_hotkey_channel = nullptr;
  if (flutter_controller_) {
    flutter_controller_ = nullptr;
  }

  Win32Window::OnDestroy();
}

LRESULT
FlutterWindow::MessageHandler(HWND hwnd, UINT const message,
                              WPARAM const wparam,
                              LPARAM const lparam) noexcept {
  // Give Flutter, including plugins, an opportunity to handle window messages.
  if (flutter_controller_) {
    std::optional<LRESULT> result =
        flutter_controller_->HandleTopLevelWindowProc(hwnd, message, wparam,
                                                      lparam);
    if (result) {
      return *result;
    }
  }

  switch (message) {
    case WM_FONTCHANGE:
      flutter_controller_->engine()->ReloadSystemFonts();
      break;
    case WM_HOTKEY:
      // RegisterHotKey 的 id 通过 wparam 传回。
      if (static_cast<int>(wparam) == kFastLedgerHotkeyId && g_hotkey_channel) {
        g_hotkey_channel->InvokeMethod("onHotkey", nullptr);
      }
      break;
  }

  return Win32Window::MessageHandler(hwnd, message, wparam, lparam);
}
