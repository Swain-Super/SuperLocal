import assert from "node:assert/strict";
import test from "node:test";
import { parsePort } from "../src/port.js";

test("1 和 65535 可以通过", () => {
  assert.deepEqual(parsePort(1), { ok: true, port: 1 });
  assert.deepEqual(parsePort("65535"), { ok: true, port: 65535 });
  assert.deepEqual(parsePort(" 8080 "), { ok: true, port: 8080 });
});

test("空、0、65536 和小数不行", () => {
  for (const value of ["", "   ", 0, "0", 65536, "65536", 8080.5, "8080.5", null]) {
    assert.equal(parsePort(value).ok, false);
  }
});
