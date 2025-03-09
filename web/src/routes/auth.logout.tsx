import { api } from "@/lib/utils/axios-api";
import { createFileRoute, redirect } from "@tanstack/solid-router";

export const Route = createFileRoute("/auth/logout")({
	component: RouteComponent,
	loader: async () => {
		await api.post<void>("/v1/auth/logout").catch(() => {});
		throw redirect({
			to: "/auth/login",
		});
	},
});

function RouteComponent() {
	return <div>Hello "/auth/logout"!</div>;
}
