import { defineConfig } from "vite";
import solid from "vite-plugin-solid";
import tsconfigPaths from "vite-tsconfig-paths";
import viteSolidFsRouter from "./plugins/fs-router";

export default defineConfig({
	plugins: [solid(), tsconfigPaths(), viteSolidFsRouter()],
});
