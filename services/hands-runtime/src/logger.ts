const SERVICE_NAME = process.env.SERVICE_NAME || "hands-runtime";

interface LogFields {
  [key: string]: unknown;
}

function emit(
  level: "info" | "warn" | "error",
  msg: string,
  fields?: LogFields
): void {
  const entry = {
    ts: new Date().toISOString(),
    level,
    service: SERVICE_NAME,
    msg,
    ...fields,
  };
  const line = JSON.stringify(entry) + "\n";
  if (level === "error") {
    process.stderr.write(line);
  } else {
    process.stdout.write(line);
  }
}

export const logger = {
  info: (msg: string, fields?: LogFields) => emit("info", msg, fields),
  warn: (msg: string, fields?: LogFields) => emit("warn", msg, fields),
  error: (msg: string, fields?: LogFields) => emit("error", msg, fields),
};
