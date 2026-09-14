"""生成 gRPC Python stub 到 rpc/gen/。

用法（在 py-backend 目录执行，可重复运行）：
    .venv\\Scripts\\python.exe tools\\gen_proto.py

生成后自动：
1. 写 rpc/gen/__init__.py（使 gen 成为包）
2. 把 dbawork_pb2_grpc.py 中的 `import dbawork_pb2` 修补为相对导入，
   保证 `from rpc.gen import dbawork_pb2_grpc` 可用。
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent      # py-backend
PROTO_DIR = ROOT.parent / "proto"                  # DBAWORK/proto
PROTO_FILE = PROTO_DIR / "dbawork.proto"
OUT_DIR = ROOT / "rpc" / "gen"


def main() -> int:
    if not PROTO_FILE.is_file():
        print(f"[gen_proto] 未找到 proto 文件: {PROTO_FILE}", file=sys.stderr)
        return 1

    OUT_DIR.mkdir(parents=True, exist_ok=True)

    # 用 grpc_tools.protoc 生成 python stub
    from grpc_tools import protoc

    args = [
        "grpc_tools.protoc",
        f"-I{PROTO_DIR}",
        f"--python_out={OUT_DIR}",
        f"--grpc_python_out={OUT_DIR}",
        str(PROTO_FILE),
    ]
    rc = protoc.main(args)
    if rc != 0:
        print(f"[gen_proto] protoc 执行失败，退出码 {rc}", file=sys.stderr)
        return rc

    # 1) 包 __init__.py
    init_file = OUT_DIR / "__init__.py"
    init_file.write_text(
        '"""gRPC 生成代码包（由 tools/gen_proto.py 生成，请勿手动修改）。"""\n',
        encoding="utf-8",
    )

    # 2) 修补 pb2_grpc 的导入为相对导入（幂等）
    grpc_file = OUT_DIR / "dbawork_pb2_grpc.py"
    text = grpc_file.read_text(encoding="utf-8")
    patched = re.sub(r"^import dbawork_pb2 as", "from . import dbawork_pb2 as", text, flags=re.M)
    if patched != text:
        grpc_file.write_text(patched, encoding="utf-8")
        print("[gen_proto] 已修补 dbawork_pb2_grpc.py 相对导入")
    print(f"[gen_proto] 生成完成: {OUT_DIR}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
