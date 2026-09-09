import test from "node:test";
import assert from "node:assert/strict";
import worker from "./src/index.js";

const envFor = (calls) => ({
  RELAY_TOKEN: "test-token",
  EMAIL: { send: async (message) => { calls.push(message); return { messageId: "test-id" }; } },
});

test("rejects requests without the relay token", async () => {
  const response = await worker.fetch(new Request("https://relay.test/v1/verification-email", { method: "POST", body: "{}" }), envFor([]));
  assert.equal(response.status, 401);
});

test("sends a verification email only from the fixed VIP sender", async () => {
  const calls = [];
  const response = await worker.fetch(new Request("https://relay.test/v1/verification-email", {
    method: "POST",
    headers: { authorization: "Bearer test-token", "content-type": "application/json" },
    body: JSON.stringify({ to: "buyer@example.com", purpose: "register", subject: "Registration code", text: "Code: 123456" }),
  }), envFor(calls));
  assert.equal(response.status, 200);
  assert.equal(calls.length, 1);
  assert.deepEqual(calls[0].from, { email: "no-reply@laoshirenvip.com", name: "老实人AI VIP" });
  assert.equal(calls[0].to, "buyer@example.com");
});

test("rejects non-verification purposes", async () => {
  const calls = [];
  const response = await worker.fetch(new Request("https://relay.test/v1/verification-email", {
    method: "POST",
    headers: { authorization: "Bearer test-token", "content-type": "application/json" },
    body: JSON.stringify({ to: "buyer@example.com", purpose: "marketing", subject: "Promo", text: "Buy now" }),
  }), envFor(calls));
  assert.equal(response.status, 400);
  assert.equal(calls.length, 0);
});

test("sends invoice PDF as a transactional attachment", async () => {
  const calls = [];
  const response = await worker.fetch(new Request("https://relay.test/v1/invoice-email", {
    method: "POST",
    headers: { authorization: "Bearer test-token", "content-type": "application/json" },
    body: JSON.stringify({ to: "buyer@example.com", subject: "Invoice", text: "Attached", filename: "invoice.pdf", pdf_base64: "JVBERi0xLjQK" }),
  }), envFor(calls));
  assert.equal(response.status, 200);
  assert.equal(calls.length, 1);
  assert.deepEqual(calls[0].from, { email: "no-reply@laoshirenvip.com", name: "老实人AI VIP" });
  assert.deepEqual(calls[0].attachments, [{ content: "JVBERi0xLjQK", filename: "invoice.pdf", type: "application/pdf", disposition: "attachment" }]);
});
