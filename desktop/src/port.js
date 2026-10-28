// parsePort 接受 1 到 65535 的整数，或只含这些数字的字符串。
export function parsePort(value) {
  if (typeof value === "number") {
    if (!Number.isInteger(value) || value < 1 || value > 65535) {
      return { ok: false };
    }
    return { ok: true, port: value };
  }
  const text = String(value ?? "").trim();
  if (!/^[0-9]+$/.test(text)) {
    return { ok: false };
  }
  const port = Number(text);
  if (port < 1 || port > 65535) {
    return { ok: false };
  }
  return { ok: true, port };
}
