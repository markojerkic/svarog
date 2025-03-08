import { Nav } from "@/components/navigation/nav";
import {
	HeadContent,
	Outlet,
	Scripts,
	createRootRouteWithContext,
} from "@tanstack/solid-router";
// import { TanStackRouterDevtools } from '@tanstack/router-devtools'

export const Route = createRootRouteWithContext()({
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
			<Nav />
			<Outlet />
			<Scripts />
			{/* <TanStackRouterDevtools /> */}
		</>
	);
}
