const { execFileSync } = require("child_process");
const fs = require("fs");
const path = require("path");

const MACHO_MAGIC = new Set(["cffaedfe", "feedfacf", "cafebabe", "bebafeca"]);

function isMachO(file) {
  let fd;
  try {
    fd = fs.openSync(file, "r");
    const buf = Buffer.alloc(4);
    const n = fs.readSync(fd, buf, 0, 4, 0);
    return n === 4 && MACHO_MAGIC.has(buf.toString("hex"));
  } catch {
    return false;
  } finally {
    if (fd !== undefined) fs.closeSync(fd);
  }
}

function walk(dir, files) {
  for (const name of fs.readdirSync(dir)) {
    const file = path.join(dir, name);
    const stat = fs.lstatSync(file);
    if (stat.isSymbolicLink()) continue;
    if (stat.isDirectory()) walk(file, files);
    else if (isMachO(file)) files.push(file);
  }
}

exports.default = async function afterPack(context) {
  if (context.electronPlatformName !== "darwin") return;
  const app = path.join(context.appOutDir, `${context.packager.appInfo.productFilename}.app`);
  const machos = [];
  walk(app, machos);
  machos.sort((a, b) => b.split(path.sep).length - a.split(path.sep).length);
  for (const file of machos) {
    execFileSync("codesign", ["--force", "--sign", "-", file], { stdio: "inherit" });
  }
  execFileSync("codesign", ["--force", "--sign", "-", app], { stdio: "inherit" });
};
