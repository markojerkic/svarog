import type { RouteDefinition } from "@solidjs/router";
import { lazy } from "solid-js";
import { route } from "./routes/index";

const routes = [
	{
		...route,
		path: "/",
		component: lazy(() => import("./routes/index.tsx")),
	},
] satisfies RouteDefinition[];

export default routes;
