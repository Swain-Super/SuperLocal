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

export function localOnlyUrls(port) {
  return {
    chatUrls: [`http://127.0.0.1:${port}/`],
    adminUrls: [`http://127.0.0.1:${port}/admin`],
  };
}

// urlsFromBanner 在某一侧没有地址时，用本机地址补上。
export function urlsFromBanner(text, port) {
  const parsed = parseBanner(text);
  const local = localOnlyUrls(port);
  return {
    chatUrls: parsed.chatUrls.length ? parsed.chatUrls : local.chatUrls,
    adminUrls: parsed.adminUrls.length ? parsed.adminUrls : local.adminUrls,
  };
}
