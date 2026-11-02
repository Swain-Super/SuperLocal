import fs from "node:fs";
import path from "node:path";
import { normalizeDataDir } from "./datadir.js";
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

function readJson(filePath) {
  try {
    const data = JSON.parse(fs.readFileSync(filePath, "utf8"));
    if (!data || typeof data !== "object") {
      return {};
    }
    return data;
  } catch {
    return {};
  }
}

// readDataDir 读取已保存的数据目录。没有或损坏时用默认目录。
export function readDataDir(jsonText, fallback) {
  try {
    const data = JSON.parse(jsonText);
    if (!data || typeof data.dataDir !== "string") {
      return path.resolve(fallback);
    }
    return normalizeDataDir(data.dataDir, fallback);
  } catch {
    return path.resolve(fallback);
  }
}

export function readDataDirFile(filePath, fallback) {
  try {
    return readDataDir(fs.readFileSync(filePath, "utf8"), fallback);
  } catch {
    return path.resolve(fallback);
  }
}

function writeLauncherFile(filePath, body) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  fs.writeFileSync(filePath, `${JSON.stringify(body)}\n`);
}

// writeSettingsFile 只在端口合法时覆盖端口，并保留数据目录。
export function writeSettingsFile(filePath, port) {
  const parsed = parsePort(port);
  if (!parsed.ok) {
    return false;
  }
  const current = readJson(filePath);
  const body = { port: parsed.port };
  if (typeof current.dataDir === "string" && current.dataDir.trim()) {
    body.dataDir = current.dataDir;
  }
  writeLauncherFile(filePath, body);
  return true;
}

export function writeDataDirFile(filePath, dataDir, fallback) {
  const next = normalizeDataDir(dataDir, fallback);
  const current = readJson(filePath);
  const parsed = parsePort(current.port);
  writeLauncherFile(filePath, {
    port: parsed.ok ? parsed.port : 8080,
    dataDir: next,
  });
  return next;
}
