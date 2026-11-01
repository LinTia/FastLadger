import 'dart:convert';

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:fastledger_client/api/api_client.dart';
import 'package:fastledger_client/core/token_store.dart';
import 'package:fastledger_client/hotkey/hotkey_service.dart';
import 'package:http/http.dart' as http;

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('HotkeyService（平台通道 mock）', () {
    test('register 成功返回 true 并透传参数', () async {
      Map<String, dynamic>? received;
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(HotkeyService.channel, (call) async {
        if (call.method == 'registerHotkey') {
          received = (call.arguments as Map).cast<String, dynamic>();
          return true;
        }
        return null;
      });
      final fired = <int>[];
      final svc = HotkeyService(onHotkey: () => fired.add(1));
      expect(await svc.register(modifier: 3, vk: 0x46), isTrue);
      expect(received?['modifier'], 3);
      expect(received?['vk'], 0x46);
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(HotkeyService.channel, null);
    });

    test('onHotkey 平台回调触发 Dart 闭包', () async {
      var fired = 0;
      HotkeyService(onHotkey: () => fired++);
      await TestDefaultBinaryMessengerBinding
          .instance.defaultBinaryMessenger
          .handlePlatformMessage(
        'fastledger/hotkey',
        const StandardMethodCodec().encodeMethodCall(
            const MethodCall('onHotkey')),
        (_) {},
      );
      expect(fired, 1);
    });

    test('screenshot 返回 PNG 字节', () async {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(HotkeyService.channel, (call) async {
        if (call.method == 'screenshot') {
          return Uint8List.fromList([0x89, 0x50, 0x4E, 0x47]);
        }
        return null;
      });
      final svc = HotkeyService(onHotkey: () {});
      final png = await svc.screenshot();
      expect(png.length, 4);
      expect(png[0], 0x89);
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(HotkeyService.channel, null);
    });

    test('register 抛 PlatformException → 返回 false（热键被占用不炸）', () async {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(HotkeyService.channel, (call) async {
        throw PlatformException(code: 'hotkey_in_use');
      });
      final svc = HotkeyService(onHotkey: () {});
      expect(await svc.register(), isFalse);
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(HotkeyService.channel, null);
    });
  });

  group('ApiClient.postMultipart（OCR recognize 通道）', () {
    test('multipart 含文件字段与 Bearer', () async {
      late http.MultipartRequest captured;
      final client = ApiClient(
        baseUrl: 'http://t',
        tokens: InMemoryTokenStore()..save('tok-m'),
        httpClient: _MultipartCapture((r) => captured = r),
      );
      final data = await client.postMultipart(
          '/api/v1/ocr/recognize', 'image', Uint8List.fromList([1, 2, 3]),
          'a.png');
      expect(data, isNull); // data:null 信封
      expect(captured.url.path, '/api/v1/ocr/recognize');
      expect(captured.headers['Authorization'], 'Bearer tok-m');
      expect(captured.files.single.field, 'image');
      expect(captured.files.single.filename, 'a.png');
      expect(await captured.files.single.finalize().toBytes(), [1, 2, 3]);
    });
  });
}

class _MultipartCapture extends http.BaseClient {
  final void Function(http.MultipartRequest) onSend;
  _MultipartCapture(this.onSend);

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    onSend(request as http.MultipartRequest);
    return http.StreamedResponse(
        Stream.value(utf8.encode(jsonEncode({'code': 0, 'msg': 'ok'}))), 200,
        headers: {'content-type': 'application/json'});
  }
}
