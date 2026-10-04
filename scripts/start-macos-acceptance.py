#!/usr/bin/env python3
"""Start an isolated manual-input session; never synthesizes input."""
import argparse
from datetime import datetime
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
APP = ROOT / "build/bin/Sidelet.app"
FIXTURE = ROOT / "build/results/MacInteractionFixture.app"


def processes():
    output = subprocess.check_output(["/bin/ps", "-axo", "pid=,command="], text=True)
    return {int(line.split(None, 1)[0]): line.split(None, 1)[1]
            for line in output.splitlines() if len(line.split(None, 1)) == 2}


def wait_ready(log, pattern, executable):
    deadline = time.monotonic() + 20
    while time.monotonic() < deadline:
        content = log.read_text() if log.exists() else ""
        found = re.search(pattern, content)
        if found:
            pid = int(found[1])
            if processes().get(pid, "").startswith(str(executable) + " "):
                return pid
            raise RuntimeError("Ready PID does not match the launched project executable")
        time.sleep(.1)
    raise RuntimeError(f"No ready record in {log}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="new directory; existing paths are refused")
    args = parser.parse_args()
    if sys.platform != "darwin":
        parser.error("Run inside a macOS graphical login session")
    binaries = [APP / "Contents/MacOS/sidelet", FIXTURE / "Contents/MacOS/fixture"]
    if not all(binary.is_file() for binary in binaries):
        parser.error("Build macOS and the interaction fixture first")
    for command in processes().values():
        if any(command == str(binary) or command.startswith(str(binary) + " ") for binary in binaries):
            parser.error("Close existing Sidelet and interaction-fixture instances first")
    results = ROOT / "build/results"
    results.mkdir(parents=True, exist_ok=True)
    if args.output:
        directory = args.output.resolve()
        directory.mkdir(parents=True, exist_ok=False)
    else:
        directory = Path(tempfile.mkdtemp(prefix="macos-manual-", dir=results))
    metadata = {
        "schema": 1, "status": "starting", "createdAt": datetime.now().astimezone().isoformat(),
        "utcOffsetSeconds": int(datetime.now().astimezone().utcoffset().total_seconds()),
        "sideletBinary": str(binaries[0]),
        "sideletSHA256": hashlib.sha256(binaries[0].read_bytes()).hexdigest(),
        "fixtureSHA256": hashlib.sha256(binaries[1].read_bytes()).hexdigest(),
        "inputOrigin": "unconfirmed", "sideletLog": "sidelet.log", "fixtureLog": "fixture.log",
        "sideletStdout": "sidelet.stdout.log", "sideletStderr": "sidelet.stderr.log",
    }
    session_path = directory / "session.json"
    def save():
        session_path.write_text(json.dumps(metadata, ensure_ascii=False, indent=2) + "\n")
    save()
    try:
        subprocess.run(["open", "-n", "--stdout", str(directory / "sidelet.stdout.log"),
                        "--stderr", str(directory / "sidelet.stderr.log"), str(APP),
                        "--args", "-spike", "-trace-focus", "-trace-pointer",
                        "-log-file", str(directory / "sidelet.log")], check=True)
        sidelet = wait_ready(directory / "sidelet.log", r"platform=darwin pid=(\d+)", binaries[0])
        metadata["sideletPID"] = sidelet
        save()
        # Wait for both real views and shortcut registration before foregrounding the fixture.
        deadline = time.monotonic() + 20
        while time.monotonic() < deadline:
            log = (directory / "sidelet.log").read_text()
            if "registration failed" in log:
                raise RuntimeError("Shortcut conflict; close this Sidelet instance and resolve the conflict")
            if all(marker in log for marker in (
                "global-shortcut registered", "regions window=stack-0", "regions window=quick"
            )):
                break
            time.sleep(.1)
        else:
            raise RuntimeError("Sidelet views did not become ready")
        subprocess.run(["open", "-n", str(FIXTURE), "--args", str(directory / "fixture.log"),
                        str(sidelet), "manual"], check=True)
        metadata["fixturePID"] = wait_ready(directory / "fixture.log", r"READY pid=(\d+)", binaries[1])
        metadata["status"] = "ready"
        save()
    except Exception as error:
        # Retain all startup evidence. Do not kill or reuse any unrelated app.
        metadata["status"] = "startup-failed"
        metadata["error"] = str(error)
        save()
        print(f"{error}\nEvidence retained: {directory}", file=sys.stderr)
        return 1
    guide = """# macOS 真实输入验收

请使用真实键盘和鼠标。测试窗口的输入是可丢弃数据。

1. 输入 before-sidelet。实按 Control + Option + T。
2. 按 ↓、Enter、E，修改标题但不保存，按 Esc 取消，再按 Esc 退出。
3. 不点击输入框、不激活测试窗口，直接输入 after-escape。
4. 保持测试窗口前台，把鼠标移到右侧标签，待其展开；不点击输入框，输入 after-hover。
5. 测试任务间隙的点击和滚轮，以及标签左侧透明区的点击和滚轮。
   透明区须仍在 Stack 的矩形内（屏幕右侧约 300px），点击其他位置不算穿透证据。
6. 点击标签打开 Passive Quick Card，确认未抢焦点、跨间隙不会提前关闭。

完成前不要切换终端来检查：终端可能改变前台状态。
完成后运行 README 中的检查命令；只有确实使用了真实键鼠才添加
--manual-input-confirmed。输入来源声明不会让缺失或失败的检查变成通过。

这个流程仅覆盖当前输入场景。多屏/拔插/普通 Space 和不同下层应用仍须另行验收；
不会自动把 P0 升级为通过。结束时关闭夹具，并从 Sidelet 菜单栏退出。
"""
    (directory / "GUIDE.md").write_text(guide)
    print(f"Ready: {directory}\nSidelet PID {sidelet}; fixture PID {metadata['fixturePID']}")
    print("Use the real keyboard/mouse now; no input was synthesized.")
    print(f"Check afterward: python3 scripts/check-macos-acceptance.py {directory}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
