import os from "node:os";

// parseBanner 从服务标准输出里取出聊天地址和后台地址。
export function parseBanner(text) {
  const chatUrls = [];
  const adminUrls = [];
  for (const line of String(text ?? "").split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed.startsWith("http://")) {
      continue;
    }
    let url;
    try {
      url = new URL(trimmed);
    } catch {
      continue;
    }
    if (url.pathname === "/admin") {
      adminUrls.push(trimmed);
    } else if (url.pathname === "/") {
      chatUrls.push(trimmed);
    }
  }
  return { chatUrls, adminUrls };
}

function isLoopback(hostname) {
  const host = String(hostname).replace(/^\[|\]$/g, "").toLowerCase();
  return host === "localhost" || host === "::1" || host === "0.0.0.0" || host.startsWith("127.");
}

function isShareable(urlText) {
  try {
    const host = new URL(urlText).hostname;
    if (isLoopback(host) || host.startsWith("169.254.") || host.toLowerCase().startsWith("fe80:")) {
      return false;
    }
    return true;
  } catch {
    return false;
  }
}

// lanUrls 列出本机局域网 IPv4 上的聊天页和后台页。
export function lanUrls(port) {
  const chatUrls = [];
  const adminUrls = [];
  const seen = new Set();
  for (const addrs of Object.values(os.networkInterfaces())) {
    for (const addr of addrs ?? []) {
      if (addr.internal || (addr.family !== "IPv4" && addr.family !== 4)) {
        continue;
      }
      if (!isShareable(`http://${addr.address}/`) || seen.has(addr.address)) {
        continue;
      }
      seen.add(addr.address);
      chatUrls.push(`http://${addr.address}:${port}/`);
      adminUrls.push(`http://${addr.address}:${port}/admin`);
    }
  }
  return { chatUrls, adminUrls };
}

function sameList(left, right) {
  if (left.length !== right.length) {
    return false;
  }
  for (let i = 0; i < left.length; i += 1) {
    if (left[i] !== right[i]) {
      return false;
    }
  }
  return true;
}

// applyLanUrls 用刚扫到的局域网地址替换当前展示。地址集合变了才标记 changed。
export function applyLanUrls(current, next) {
  const chatUrls = next?.chatUrls ?? [];
  const adminUrls = next?.adminUrls ?? [];
  const changed = !sameList(current?.chatUrls ?? [], chatUrls) || !sameList(current?.adminUrls ?? [], adminUrls);
  return { changed, chatUrls, adminUrls };
}

// urlsFromBanner 只保留局域网地址。port 保留给调用方，展示不再补 127.0.0.1。
export function urlsFromBanner(text, port) {
  void port;
  const parsed = parseBanner(text);
  return {
    chatUrls: parsed.chatUrls.filter(isShareable),
    adminUrls: parsed.adminUrls.filter(isShareable),
  };
}
