import type { RouteDefinition } from "@solidjs/router";
import { lazy } from "solid-js";
import { route as indexRoute } from "@/routes/__authenticated/index";
import { route as clientLogsRoute } from "@/routes/logs/[clientId]/index";
import { useQueryClient } from "@tanstack/solid-query";
import { currentUserQueryOptions } from "./lib/hooks/auth/use-current-user";
import ProtectedLayout from "./routes/protected-layout";
import { adminRoute } from "@/routes/admin/index.tsx";

const routes = [
	// Protected routes
	{
		path: "/",
		preload: async () => {
			const queryClient = useQueryClient();

			return await queryClient.prefetchQuery(currentUserQueryOptions);
		},
		component: ProtectedLayout,
		children: [
			{
				path: "",
				...indexRoute,
				component: lazy(() => import("@/routes/__authenticated/index")),
			},
			{
				path: "/logs",
				children: [
					{
						path: "/:clientId",
						component: lazy(() => import("@/routes/logs/[clientId].tsx")),
						children: [
							{
								...clientLogsRoute,
								path: "/",
								component: lazy(
									() => import("@/routes/logs/[clientId]/index.tsx"),
								),
							},
							{
								path: "/search",
								component: lazy(
									() => import("@/routes/logs/[clientId]/search.tsx"),
								),
							},
						],
					},
				],
			},

			{
				path: "/admin",
				component: lazy(() => import("@/routes/admin/index.tsx")),
				...adminRoute,
			},
		],
	},
	{
		path: "/auth/login",
		children: [
			{
				path: "/",
				component: lazy(() => import("@/routes/auth/login.tsx")),
			},
			{
				path: "/:token",
				component: lazy(() => import("@/routes/auth/login-with-token.tsx")),
			},
		],
	},
	{
		path: "/auth/reset-password",
		component: lazy(() => import("@/routes/auth/reset-password")),
	},
	{
		path: "/auth/logout",
		component: lazy(() => import("@/routes/auth/logout.tsx")),
	},
	{
		path: "*404",
		component: lazy(() => import("@/routes/[...404].tsx")),
	},
] satisfies RouteDefinition[];

export default routes;
