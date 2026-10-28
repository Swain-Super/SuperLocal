import assert from "node:assert/strict";
import test from "node:test";
import { localOnlyUrls, parseBanner, urlsFromBanner } from "../src/banner.js";

const sample = `http://127.0.0.1:9090/
http://127.0.0.1:9090/admin
http://192.168.1.8:9090/
http://192.168.1.8:9090/admin
账号 admin
密码 secret
`;

test("从服务打印的文本取出聊天地址和后台地址", () => {
  const parsed = parseBanner(sample);
  assert.deepEqual(parsed.chatUrls, [
    "http://127.0.0.1:9090/",
    "http://192.168.1.8:9090/",
  ]);
  assert.deepEqual(parsed.adminUrls, [
    "http://127.0.0.1:9090/admin",
    "http://192.168.1.8:9090/admin",
  ]);
});

test("没有地址时补上本机地址", () => {
  assert.deepEqual(urlsFromBanner("账号 admin\n密码 secret\n", 9090), localOnlyUrls(9090));
});

test("采纳残留进程时只用本机地址", () => {
  assert.deepEqual(localOnlyUrls(8080), {
    chatUrls: ["http://127.0.0.1:8080/"],
    adminUrls: ["http://127.0.0.1:8080/admin"],
  });
});
