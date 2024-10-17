import solid from "vite-plugin-solid";
import viteConfig from "./app.config";
import { defineConfig, mergeConfig } from "vitest/config";
import path from "node:path";

export default mergeConfig(
    viteConfig,
    defineConfig({
        plugins: [solid()],
        resolve: {
            conditions: ["development", "browser"],
            alias: {
                "~": path.resolve(__dirname, "src"),
            },
        },
    }),
);
