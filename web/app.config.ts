/// <reference types="vitest" />
import { defineConfig } from "@solidjs/start/config";

export default defineConfig({
	ssr: false,
	devOverlay: false,
	server: {
		preset: "static",
	},
	solid: {
		ssr: false,
	},
});
