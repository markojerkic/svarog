import type { LoggedInUser } from "@/lib/hooks/auth/use-current-user";
import type { QueryClient } from "@tanstack/solid-query";
import {
	HeadContent,
	Outlet,
	Scripts,
	createRootRouteWithContext,
} from "@tanstack/solid-router";
// import { TanStackRouterDevtools } from '@tanstack/router-devtools'

interface RouterContext {
	auth?: LoggedInUser;
	queryClient: QueryClient;
}

export const Route = createRootRouteWithContext<RouterContext>()({
	component: RootComponent,
	head: () => ({
		meta: [
			{ title: "Svarog" },
			{ name: "description", content: "Log aggregation and monitoring tool" },
		],
	}),
});

function RootComponent() {
	return (
		<>
			<HeadContent />
			<Outlet />
			<Scripts />
			{/* <TanStackRouterDevtools /> */}
		</>
	);
}
