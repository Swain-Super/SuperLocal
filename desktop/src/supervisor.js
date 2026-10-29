export function explainStartupError(text) {
  const line = lastNonEmptyLine(text);
  if (/address already in use|EADDRINUSE/i.test(line)) {
    return "端口已被占用";
  }
  if (/EACCES|permission denied/i.test(line)) {
    return "没有权限监听这个端口";
  }
  return line || "服务启动失败";
}

export function lastNonEmptyLine(text) {
  const lines = String(text ?? "")
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.length ? lines[lines.length - 1] : "";
}

export async function pollReady({ probe, shouldStop, timeoutMs, delay }) {
  let elapsed = 0;
  while (elapsed < timeoutMs) {
    if (shouldStop()) {
      return "stopped";
    }
    if (await probe()) {
      return "ready";
    }
    const step = Math.min(200, timeoutMs - elapsed);
    await delay(step);
    elapsed += step;
  }
  if (shouldStop()) {
    return "stopped";
  }
  if (await probe()) {
    return "ready";
  }
  return "timeout";
}

export function outcomeFromPoll(result, stderr) {
  if (result === "stopped") {
    return { status: "stopped", error: "", kill: true };
  }
  if (result === "ready") {
    return { status: "running", error: "", kill: false };
  }
  if (result === "exit") {
    return {
      status: "failed",
      error: lastNonEmptyLine(stderr) || "服务启动失败",
      kill: false,
    };
  }
  return {
    status: "failed",
    error: "服务在 10 秒内没有开始监听",
    kill: true,
  };
}

export async function watchStartup({
  exited,
  probe,
  shouldStop,
  timeoutMs,
  delay,
  readStderr,
}) {
  let finished = false;
  const exitResult = exited.then((stderr) => {
    finished = true;
    return { kind: "exit", stderr: stderr ?? readStderr() };
  });
  const pollResult = pollReady({
    probe,
    shouldStop: () => finished || shouldStop(),
    timeoutMs,
    delay,
  }).then((result) => ({ kind: "poll", result }));
  const winner = await Promise.race([exitResult, pollResult]);
  if (shouldStop()) {
    return outcomeFromPoll("stopped", "");
  }
  if (winner.kind === "exit") {
    return outcomeFromPoll("exit", winner.stderr);
  }
  return outcomeFromPoll(winner.result, readStderr());
}

export async function requestStop({ signal, isAlive, delay, waitMs = 5000 }) {
  signal("term");
  let elapsed = 0;
  while (elapsed < waitMs) {
    if (!isAlive()) {
      return { ok: true, error: "" };
    }
    const step = Math.min(100, waitMs - elapsed);
    await delay(step);
    elapsed += step;
  }
  if (!isAlive()) {
    return { ok: true, error: "" };
  }
  signal("kill");
  await delay(50);
  if (!isAlive()) {
    return { ok: true, error: "" };
  }
  return { ok: false, error: "停止服务失败" };
}
