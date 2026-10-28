// decidePid 只采纳仍在运行、且可执行文件就是本次服务程序的进程。
export function decidePid(probe, expectedExe) {
  if (!probe || !probe.alive || !probe.exePath || !expectedExe) {
    return "discard";
  }
  if (probe.exePath !== expectedExe) {
    return "discard";
  }
  return "adopt";
}

export function parsePid(text) {
  const value = String(text ?? "").trim();
  if (!/^[1-9][0-9]*$/.test(value)) {
    return null;
  }
  const pid = Number(value);
  if (!Number.isSafeInteger(pid)) {
    return null;
  }
  return pid;
}

// exeFromCommand 取命令行的第一个参数。带引号的路径保留其中的空格。
export function exeFromCommand(commandLine) {
  const cmd = String(commandLine ?? "").trim();
  if (!cmd) {
    return null;
  }
  if (cmd.startsWith('"')) {
    const end = cmd.indexOf('"', 1);
    if (end > 1) {
      return cmd.slice(1, end);
    }
    return null;
  }
  const token = cmd.split(/\s+/)[0];
  return token || null;
}
