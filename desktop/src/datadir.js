import fs from "node:fs";
import os from "node:os";
import path from "node:path";

// normalizeDataDir 把用户输入收成绝对路径。空字符串用默认目录。
export function normalizeDataDir(raw, fallback) {
  const text = String(raw ?? "").trim();
  const base = path.resolve(fallback);
  if (!text) {
    return base;
  }
  if (text === "~") {
    return os.homedir();
  }
  if (text.startsWith("~/") || text.startsWith("~\\")) {
    return path.resolve(path.join(os.homedir(), text.slice(2)));
  }
  return path.resolve(text);
}

function overlaps(left, right) {
  const rel = path.relative(left, right);
  return rel === "" || (!rel.startsWith("..") && !path.isAbsolute(rel));
}

// moveDataDir 把现有数据目录整份搬到新路径。目标里已有文件时拒绝。
export function moveDataDir(current, next) {
  const from = path.resolve(current);
  const to = path.resolve(next);
  if (from === to) {
    return { ok: true, dataDir: to, error: "" };
  }
  if (overlaps(from, to) || overlaps(to, from)) {
    return { ok: false, dataDir: from, error: "目标路径和当前数据目录重叠" };
  }
  let fromExists = false;
  try {
    fromExists = fs.existsSync(from);
  } catch {
    return { ok: false, dataDir: from, error: "无法读取当前数据目录" };
  }
  if (!fromExists) {
    return { ok: true, dataDir: to, error: "" };
  }
  let fromStat;
  try {
    fromStat = fs.statSync(from);
  } catch {
    return { ok: false, dataDir: from, error: "无法读取当前数据目录" };
  }
  if (!fromStat.isDirectory()) {
    return { ok: false, dataDir: from, error: "当前数据目录不可用" };
  }
  let removedEmpty = false;
  try {
    if (fs.existsSync(to)) {
      const toStat = fs.statSync(to);
      if (!toStat.isDirectory()) {
        return { ok: false, dataDir: from, error: "目标路径不是目录" };
      }
      if (fs.readdirSync(to).length > 0) {
        return { ok: false, dataDir: from, error: "目标目录不是空的" };
      }
      fs.rmdirSync(to);
      removedEmpty = true;
    } else {
      fs.mkdirSync(path.dirname(to), { recursive: true });
    }
    fs.renameSync(from, to);
  } catch (err) {
    if (err && err.code === "EXDEV") {
      try {
        fs.cpSync(from, to, { recursive: true });
        fs.rmSync(from, { recursive: true, force: true });
        return { ok: true, dataDir: to, error: "" };
      } catch {
        return { ok: false, dataDir: from, error: "无法移动数据目录" };
      }
    }
    if (removedEmpty) {
      try {
        fs.mkdirSync(to);
      } catch {
        // 空目录已经删掉，恢复失败时仍把错误告诉用户。
      }
    }
    return { ok: false, dataDir: from, error: "无法移动数据目录" };
  }
  return { ok: true, dataDir: to, error: "" };
}
