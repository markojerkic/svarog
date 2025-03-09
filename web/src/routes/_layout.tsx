
import { createFileRoute, Outlet } from "@tanstack/solid-router";

export const Route = createFileRoute("/_layout")({
	component: RouteComponent,
});

function RouteComponent() {
	return (
		<div class="flex h-screen flex-col">
			<Outlet />
		</div>
	);
}
