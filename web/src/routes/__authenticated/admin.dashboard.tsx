import { createFileRoute } from "@tanstack/solid-router";

export const Route = createFileRoute("/__authenticated/admin/dashboard")({
	component: RouteComponent,
});

function RouteComponent() {
	return <div>Hello "/__authenticated/admin/"!</div>;
}
