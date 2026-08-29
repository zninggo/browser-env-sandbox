"""真实 bes-server 冒烟测试:Call 成功路径。

前置:bes-server 已在 BES_TEST_ADDR(默认 localhost:18080)监听且无 auth。
  BES_TEST_ADDR=127.0.0.1:18080 python sdk/python/test_call_smoke.py

覆盖契约:SDK 发送 {"function":...,"args":[...]}(见 bes/__init__.py:149),
bridge callRequest.Function(json tag "function")匹配,
/call 返回 200 且 result 为函数返回值。
"""
import os
import socket
import sys

sys.path.insert(0, os.path.dirname(__file__))
from bes import Sandbox, BESError  # noqa: E402


def probe(addr: str) -> bool:
    host, _, port = addr.partition(":")
    try:
        with socket.create_connection((host, int(port)), timeout=1):
            return True
    except OSError:
        return False


def main() -> int:
    addr = os.environ.get("BES_TEST_ADDR", "localhost:18080")
    if not probe(addr):
        print(f"SKIP: bes-server not reachable at {addr}")
        return 0

    try:
        s = Sandbox(server_addr=addr)
    except BESError as e:
        print(f"FAIL: CreateSession failed: {e}")
        return 1
    try:
        s.eval("function add(a,b){return String(Number(a)+Number(b))}")
        got = s.call("add", "1", "2")
        want = "3"
        if got != want:
            print(f"FAIL: Call result = {got!r}, want {want!r}")
            return 1
        print(f"Call(add,1,2) = {got!r} ✓")
    except BESError as e:
        print(f"FAIL: {e} (contract broken? SDK must send 'function' not 'function_name')")
        return 1
    finally:
        s.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
