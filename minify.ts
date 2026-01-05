#!/usr/bin/env bun

import { Glob } from "bun";

const glob = new Glob("internal/server/ui/assets/js/*.min.js");

type Entry = {
  fullPath: string;
  minPath: string;
};
const entiypoints: Entry[] = [];

for await (const entry of glob.scan()) {
  const fullFile = entry.replace(".min", "");
  // if fullFile exists
  if (!(await Bun.file(fullFile).exists())) {
    continue;
  }
  entiypoints.push({
    fullPath: fullFile,
    minPath: entry,
  });
}

const resp = await Bun.build({
  entrypoints: entiypoints.map((e) => e.fullPath),
  outdir: "internal/server/ui/assets/js",
  naming: "[name].min.js",
  minify: true,
});

console.log(resp);
