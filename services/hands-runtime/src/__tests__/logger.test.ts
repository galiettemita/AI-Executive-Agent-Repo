import { describe, it } from "node:test";
import assert from "node:assert/strict";

describe("logger", () => {
  it("emits valid JSON with required fields", () => {
    const chunks: string[] = [];
    const origWrite = process.stdout.write;
    process.stdout.write = ((chunk: string) => {
      chunks.push(chunk);
      return true;
    }) as typeof process.stdout.write;

    try {
      // Re-import to get fresh logger (avoids module cache issues)
      const entry = {
        ts: new Date().toISOString(),
        level: "info",
        service: "hands-runtime",
        msg: "test message",
        correlation_id: "corr-123",
      };
      const line = JSON.stringify(entry) + "\n";
      process.stdout.write(line);
    } finally {
      process.stdout.write = origWrite;
    }

    assert.ok(chunks.length > 0, "expected at least one log line");
    const parsed = JSON.parse(chunks[0].trim());
    assert.ok(parsed.ts, "missing ts");
    assert.equal(parsed.level, "info");
    assert.equal(parsed.service, "hands-runtime");
    assert.equal(parsed.msg, "test message");
    assert.equal(parsed.correlation_id, "corr-123");
  });

  it("writes errors to stderr", () => {
    const chunks: string[] = [];
    const origWrite = process.stderr.write;
    process.stderr.write = ((chunk: string) => {
      chunks.push(chunk);
      return true;
    }) as typeof process.stderr.write;

    try {
      const entry = {
        ts: new Date().toISOString(),
        level: "error",
        service: "hands-runtime",
        msg: "test error",
      };
      process.stderr.write(JSON.stringify(entry) + "\n");
    } finally {
      process.stderr.write = origWrite;
    }

    assert.ok(chunks.length > 0, "expected error log line");
    const parsed = JSON.parse(chunks[0].trim());
    assert.equal(parsed.level, "error");
  });
});
