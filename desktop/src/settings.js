import fs from "node:fs";
import { parsePort } from "./port.js";

// readSettings 只接受 JSON 里的数字端口。缺失、损坏或非法时返回 8080。
export function readSettings(jsonText) {
  try {
    const data = JSON.parse(jsonText);
    if (!data || typeof data.port !== "number") {
      return 8080;
    }
    const parsed = parsePort(data.port);
    return parsed.ok ? parsed.port : 8080;
  } catch {
    return 8080;
  }
}

export function readSettingsFile(filePath) {
  try {
    return readSettings(fs.readFileSync(filePath, "utf8"));
  } catch {
    return 8080;
  }
}

// writeSettingsFile 只在端口合法时覆盖文件。
export function writeSettingsFile(filePath, port) {
  const parsed = parsePort(port);
  if (!parsed.ok) {
    return false;
  }
  fs.writeFileSync(filePath, `${JSON.stringify({ port: parsed.port })}\n`);
  return true;
}
