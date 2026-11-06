import assert from "node:assert/strict";
import test from "node:test";
import { applyLanUrls, lanUrls, parseBanner, urlsForHosts, urlsFromBanner } from "../src/banner.js";

const sample = `https://127.0.0.1:9090/
https://127.0.0.1:9090/admin
https://192.168.1.8:9090/
https://192.168.1.8:9090/admin
账号 admin
密码 secret
`;

test("从服务打印的文本取出聊天地址和后台地址", () => {
  const parsed = parseBanner(sample);
  assert.deepEqual(parsed.chatUrls, [
    "https://127.0.0.1:9090/",
    "https://192.168.1.8:9090/",
  ]);
  assert.deepEqual(parsed.adminUrls, [
    "https://127.0.0.1:9090/admin",
    "https://192.168.1.8:9090/admin",
  ]);
});

test("旧的 http 地址仍然能解析", () => {
  const parsed = parseBanner("http://192.168.1.8:9090/\nhttp://192.168.1.8:9090/admin\n");
  assert.deepEqual(parsed.chatUrls, ["http://192.168.1.8:9090/"]);
  assert.deepEqual(parsed.adminUrls, ["http://192.168.1.8:9090/admin"]);
});

test("页面只展示局域网地址", () => {
  const text = `${sample}http://169.254.95.6:9090/\nhttp://169.254.95.6:9090/admin\n`;
  assert.deepEqual(urlsFromBanner(text, 9090), {
    chatUrls: ["https://192.168.1.8:9090/"],
    adminUrls: ["https://192.168.1.8:9090/admin"],
  });
});

test("没有局域网地址时不补回环地址", () => {
  assert.deepEqual(urlsFromBanner("账号 admin\n密码 secret\n", 9090), {
    chatUrls: [],
    adminUrls: [],
  });
});

test("地址没变时不标记切换", () => {
  const urls = {
    chatUrls: ["http://192.168.2.56:8080/"],
    adminUrls: ["http://192.168.2.56:8080/admin"],
  };
  const applied = applyLanUrls(urls, urls);
  assert.equal(applied.changed, false);
  assert.deepEqual(applied.chatUrls, urls.chatUrls);
  assert.deepEqual(applied.adminUrls, urls.adminUrls);
});

test("切换 IP 后换成新地址", () => {
  const applied = applyLanUrls(
    {
      chatUrls: ["http://192.168.2.56:8080/"],
      adminUrls: ["http://192.168.2.56:8080/admin"],
    },
    {
      chatUrls: ["http://10.0.0.8:8080/"],
      adminUrls: ["http://10.0.0.8:8080/admin"],
    },
  );
  assert.equal(applied.changed, true);
  assert.deepEqual(applied.chatUrls, ["http://10.0.0.8:8080/"]);
  assert.deepEqual(applied.adminUrls, ["http://10.0.0.8:8080/admin"]);
});

test("每个局域网地址先列 https 再列 http", () => {
  assert.deepEqual(urlsForHosts(8080, ["192.168.1.8", "10.0.0.2"]), {
    chatUrls: [
      "https://192.168.1.8:8080/",
      "http://192.168.1.8:8080/",
      "https://10.0.0.2:8080/",
      "http://10.0.0.2:8080/",
    ],
    adminUrls: [
      "https://192.168.1.8:8080/admin",
      "http://192.168.1.8:8080/admin",
      "https://10.0.0.2:8080/admin",
      "http://10.0.0.2:8080/admin",
    ],
  });
});

test("局域网的 http 和 https 都保留", () => {
  const text = [
    "https://192.168.1.8:9090/",
    "https://192.168.1.8:9090/admin",
    "http://192.168.1.8:9090/",
    "http://192.168.1.8:9090/admin",
    "http://127.0.0.1:9090/",
    "http://127.0.0.1:9090/admin",
  ].join("\n");
  assert.deepEqual(urlsFromBanner(text, 9090), {
    chatUrls: ["https://192.168.1.8:9090/", "http://192.168.1.8:9090/"],
    adminUrls: ["https://192.168.1.8:9090/admin", "http://192.168.1.8:9090/admin"],
  });
});

test("采纳残留进程时列出局域网地址", () => {
  const urls = lanUrls(8080);
  for (const url of [...urls.chatUrls, ...urls.adminUrls]) {
    const host = new URL(url).hostname;
    assert.equal(host.startsWith("127."), false);
    assert.equal(host.startsWith("169.254."), false);
  }
});
