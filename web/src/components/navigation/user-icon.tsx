import type { RouteDefinition } from "@solidjs/router";
import { lazy } from "solid-js";
import { route as indexRoute } from "./routes/index";
import { route as clientLogsRoute } from "./routes/logs/[clientId]/index";

const routes = [
	{
		...indexRoute,
		path: "/",
		component: lazy(() => import("./routes/index.tsx")),
	},
	{
		path: "/logs",
		children: [
			{
				path: "/:clientId",
				component: lazy(() => import("./routes/logs/[clientId].tsx")),
				children: [
					{
						...clientLogsRoute,
						path: "/",
						component: lazy(() => import("./routes/logs/[clientId]/index.tsx")),
					},
					{
						path: "/search",
						component: lazy(
							() => import("./routes/logs/[clientId]/search.tsx"),
						),
					},
				],
			},
		],
	},
	{
		path: "*404",
		component: lazy(() => import("./routes/[...404].tsx")),
	},
] satisfies RouteDefinition[];

export default routes;
