#!/usr/bin/env bun

// Tells the templ proxy to reload the browser when Tailwind or an app script
// changes. Component bundles are left out: their URL is compiled into the
// server, so a reload before that restart would request a hash that is gone.
import { watch, type WatchEventType } from "node:fs";

const proxyPort = process.env.PROXY_PORT ?? "7331";
const cssDir = "internal/server/ui/assets/css";
const jsDir = "internal/server/ui/assets/js";

let pending: ReturnType<typeof setTimeout> | undefined;

function schedule(): void {
  if (pending !== undefined) {
    clearTimeout(pending);
  }
  pending = setTimeout(() => {
    pending = undefined;
    notify();
  }, 100);
}

function notify(): void {
  const proc = Bun.spawn(
    ["go", "tool", "templ", "generate", "--notify-proxy", `--proxyport=${proxyPort}`],
    { stdout: "ignore", stderr: "ignore" },
  );
  void proc.exited.then((code) => {
    if (code === 0) {
      console.log("reloaded browser");
    }
  });
}

function relevant(kind: "css" | "js", filename: string | null): boolean {
  if (filename === null || filename.includes("/")) {
    return false;
  }
  if (kind === "css") {
    return filename === "output.css";
  }
  return filename.endsWith(".js") && !filename.startsWith("shadcn-templ-");
}

function watchDir(dir: string, kind: "css" | "js"): void {
  watch(dir, (_event: WatchEventType, filename) => {
    const name = filename === null ? null : filename.toString();
    if (!relevant(kind, name)) {
      return;
    }
    schedule();
  });
}

watchDir(cssDir, "css");
watchDir(jsDir, "js");
console.log(`watching ${cssDir}/output.css and ${jsDir}/*.js`);
