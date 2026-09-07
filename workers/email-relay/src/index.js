const sender = { email: "no-reply@laoshirenvip.com", name: "lsrai.shop" };
const allowedPurposes = new Set(["register", "reset", "telegram_bind", "change_email_old", "change_email_new"]);
const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

const json = (body, status = 200) => new Response(JSON.stringify(body), {
  status,
  headers: { "content-type": "application/json; charset=utf-8", "cache-control": "no-store" },
});

const digest = async (value) => {
  const bytes = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(value));
  return Array.from(new Uint8Array(bytes), (byte) => byte.toString(16).padStart(2, "0")).join("");
};

const authorized = async (request, env) => {
  const expected = String(env.RELAY_TOKEN || "").trim();
  const header = request.headers.get("authorization") || "";
  const provided = header.startsWith("Bearer ") ? header.slice(7).trim() : "";
  if (!expected || !provided) return false;
  const [left, right] = await Promise.all([digest(provided), digest(expected)]);
  return left === right;
};

const cleanLine = (value, max) => {
  const text = String(value || "").replace(/[\r\n\0]/g, " ").trim();
  return text.length <= max ? text : text.slice(0, max);
};

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (request.method !== "POST" || url.pathname !== "/v1/verification-email") {
      return json({ ok: false, error: "not_found" }, 404);
    }
    if (!(await authorized(request, env))) {
      return json({ ok: false, error: "unauthorized" }, 401);
    }
    if (Number(request.headers.get("content-length") || "0") > 16384) {
      return json({ ok: false, error: "payload_too_large" }, 413);
    }
    let input;
    try {
      input = await request.json();
    } catch {
      return json({ ok: false, error: "invalid_json" }, 400);
    }
    const to = cleanLine(input?.to, 320).toLowerCase();
    const purpose = cleanLine(input?.purpose, 40).toLowerCase();
    const subject = cleanLine(input?.subject, 200);
    const text = String(input?.text || "").replace(/\0/g, "").trim().slice(0, 8000);
    if (!emailPattern.test(to) || !allowedPurposes.has(purpose) || !subject || !text) {
      return json({ ok: false, error: "invalid_payload" }, 400);
    }
    try {
      const result = await env.EMAIL.send({ to, from: sender, subject, text });
      return json({ ok: true, accepted: true, message_id_present: Boolean(result?.messageId) });
    } catch (error) {
      console.error("verification_email_send_failed", { code: cleanLine(error?.code, 80) });
      return json({ ok: false, error: "send_failed" }, 502);
    }
  },
};
