import type { ApiError } from "@/lib/errors/api-error";
import type { LoggedInUser } from "@/lib/hooks/auth/use-current-user";
import type { QueryClient } from "@tanstack/solid-query";
import {
	HeadContent,
	Outlet,
	Scripts,
	createRootRouteWithContext,
} from "@tanstack/solid-router";
import { TanStackRouterDevtools } from "@tanstack/solid-router-devtools";

interface RouterContext {
	auth?: LoggedInUser;
	authError?: ApiError | null;
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
			<TanStackRouterDevtools />
		</>
	);
}
