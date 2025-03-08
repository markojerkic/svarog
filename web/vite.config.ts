import { defineConfig } from "vite";
import { TanStackRouterVite } from "@tanstack/router-plugin/vite";
import solid from "vite-plugin-solid";
import tsconfigPaths from "vite-tsconfig-paths";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
	plugins: [
		TanStackRouterVite({ target: "solid", autoCodeSplitting: true }),
		solid(),
		tsconfigPaths(),
		tailwindcss(),
	],
});
