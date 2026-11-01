#include "hotkey_handler.h"

#include <windows.h>
#include <gdiplus.h>
#include <shlwapi.h>

#include <flutter/encodable_value.h>
#include <flutter/method_result.h>

#include <atomic>
#include <vector>

std::unique_ptr<flutter::MethodChannel<flutter::EncodableValue>>
    g_hotkey_channel;

namespace {

constexpr int kHotkeyId = 1;

std::atomic<bool> g_registered{false};

// GDI+ 一次性初始化（进程级）。
struct GdiPlusBoot {
  ULONG_PTR token = 0;
  GdiPlusBoot() { GdiplusStartup(&token, nullptr, nullptr); }
  ~GdiPlusBoot() {
    if (token) GdiplusShutdown(token);
  }
};
GdiPlusBoot& GdiPlus() {
  static GdiPlusBoot boot;
  return boot;
}

// GetImageEncoders 查找 PNG 编码器的 CLSID。
bool FindPngEncoderClsid(CLSID* out) {
  UINT num = 0, size = 0;
  if (Gdiplus::GetImageEncodersSize(&num, &size) != Gdiplus::Ok || size == 0) {
    return false;
  }
  std::vector<char> buf(size);
  auto* encoders = reinterpret_cast<Gdiplus::ImageCodecInfo*>(buf.data());
  if (Gdiplus::GetImageEncoders(num, size, encoders) != Gdiplus::Ok) {
    return false;
  }
  for (UINT i = 0; i < num; ++i) {
    if (wcscmp(encoders[i].MimeType, L"image/png") == 0) {
      *out = encoders[i].Clsid;
      return true;
    }
  }
  return false;
}

}  // namespace

bool HotkeyRegister(HWND hwnd, int modifier, int vk) {
  if (g_registered.load()) {
    UnregisterHotKey(hwnd, kHotkeyId);
    g_registered.store(false);
  }
  if (!RegisterHotKey(hwnd, kHotkeyId, static_cast<UINT>(modifier),
                      static_cast<UINT>(vk))) {
    return false;
  }
  g_registered.store(true);
  return true;
}

void HotkeyUnregister(HWND hwnd) {
  if (g_registered.load()) {
    UnregisterHotKey(hwnd, kHotkeyId);
    g_registered.store(false);
  }
}

std::vector<uint8_t> CaptureForegroundWindowPng() {
  GdiPlus();
  HWND hwnd = GetForegroundWindow();
  if (!hwnd) return {};

  RECT rect{};
  if (!GetWindowRect(hwnd, &rect)) return {};
  const int width = rect.right - rect.left;
  const int height = rect.bottom - rect.top;
  if (width <= 0 || height <= 0) return {};

  HDC window_dc = GetWindowDC(hwnd);
  if (!window_dc) return {};
  HDC mem_dc = CreateCompatibleDC(window_dc);
  HBITMAP bitmap = CreateCompatibleBitmap(window_dc, width, height);
  HGDIOBJ old = SelectObject(mem_dc, bitmap);

  // PW_RENDERFULLCONTENT：含 DirectComposition 内容（Chrome/Edge 等）。
  BOOL printed = PrintWindow(hwnd, mem_dc, PW_CLIENTONLY | PW_RENDERFULLCONTENT);
  std::vector<uint8_t> png;
  if (printed) {
    Gdiplus::Bitmap image(bitmap, nullptr);
    CLSID clsid{};
    if (image.GetLastStatus() == Gdiplus::Ok && FindPngEncoderClsid(&clsid)) {
      IStream* stream = nullptr;
      if (CreateStreamOnHGlobal(nullptr, TRUE, &stream) == S_OK) {
        if (image.Save(stream, &clsid) == Gdiplus::Ok) {
          HGLOBAL hglob = nullptr;
          if (GetHGlobalFromStream(stream, &hglob) == S_OK) {
            const SIZE_T size = GlobalSize(hglob);
            const void* data = GlobalLock(hglob);
            if (data) {
              // GlobalSize 可大于实际流长度；GDI+ 保存后的 PNG 以 IEnd 结尾，
              // 直接复制即可（解码器容忍尾部填充）。
              png.assign(static_cast<const uint8_t*>(data),
                         static_cast<const uint8_t*>(data) + size);
              GlobalUnlock(hglob);
            }
          }
        }
        stream->Release();
      }
    }
  }

  SelectObject(mem_dc, old);
  DeleteObject(bitmap);
  DeleteDC(mem_dc);
  ReleaseDC(hwnd, window_dc);
  return png;
}
