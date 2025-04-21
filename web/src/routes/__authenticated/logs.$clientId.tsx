import { createFileRoute, Outlet } from "@tanstack/solid-router";

export const Route = createFileRoute("/__authenticated/logs/$clientId")({
	component: RouteComponent,
	head: ({ params }) => ({
		meta: [{ title: `Client: ${params.clientId}` }],
	}),
});

function RouteComponent() {
	return (
		<div class="grow">
			<Outlet />
		</div>
	);
}
