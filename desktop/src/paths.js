import path from "node:path";

export function serviceBinaryName(platform = process.platform) {
  return platform === "win32" ? "superlocal.exe" : "superlocal";
}

export function serviceBinaryPath({
  isPackaged,
  resourcesPath,
  appRoot,
  platform = process.platform,
}) {
  const name = serviceBinaryName(platform);
  if (isPackaged) {
    return path.join(resourcesPath, name);
  }
  return path.join(appRoot, "binaries", name);
}

export function launcherPaths(userData) {
  return {
    settings: path.join(userData, "launcher.json"),
    pid: path.join(userData, "superlocal.pid"),
    dataDir: path.join(userData, "data"),
  };
}
