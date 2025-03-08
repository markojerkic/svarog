import { createFileRoute } from "@tanstack/solid-router";

export const Route = createFileRoute("/logs/$clientId/")({
	component: RouteComponent,
});

function RouteComponent() {
	return <div>Hello "/logs/$clientId/"!</div>;
}
