import { createFileRoute } from "@tanstack/solid-router";
import { useLogout } from "@/lib/hooks/auth/login";
import { onMount } from "solid-js";

export const Route = createFileRoute("/auth/logout")({
	component: RouteComponent,
});

function RouteComponent() {
	const logout = useLogout();

	onMount(() => {
		logout.mutate();
	});

	return <div>Logging out...</div>;
}
