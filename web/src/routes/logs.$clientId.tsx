import { createFileRoute, Outlet } from "@tanstack/solid-router";

export const Route = createFileRoute("/logs/$clientId")({
	component: RouteComponent,
	head: ({ params }) => ({
		meta: [{ title: `Client: ${params.clientId}` }],
	}),
});

function RouteComponent() {
	return (
		<div class="grow overflow-y-hidden">
			<Outlet />
		</div>
	);
}
