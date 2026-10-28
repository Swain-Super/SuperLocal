import fs from "node:fs";
import path from "node:path";

// parseSecretText 要求去掉末尾换行后恰好是两行非空文本。
export function parseSecretText(text) {
  const lines = String(text ?? "")
    .replace(/\r/g, "")
    .replace(/\n$/, "")
    .split("\n");
  if (lines.length !== 2 || lines[0] === "" || lines[1] === "") {
    return { secretState: "unreadable", username: "", password: "" };
  }
  return { secretState: "ready", username: lines[0], password: lines[1] };
}

export function readSecretFile(dataDir) {
  const filePath = path.join(dataDir, "admin.secret");
  let text;
  try {
    text = fs.readFileSync(filePath, "utf8");
  } catch (err) {
    if (err && err.code === "ENOENT") {
      return { secretState: "hidden", username: "", password: "" };
    }
    return { secretState: "unreadable", username: "", password: "" };
  }
  return parseSecretText(text);
}
