import viteConfig from "./vite.config";
import { defineConfig, mergeConfig } from "vitest/config";
import path from "node:path";

export default mergeConfig(
	viteConfig,
	defineConfig({
		resolve: {
			conditions: ["development", "browser"],
			alias: {
				"@": path.resolve(__dirname, "src"),
			},
		},
	}),
);
