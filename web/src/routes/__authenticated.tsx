import { Nav } from "@/components/navigation/nav";
import { ApiError } from "@/lib/errors/api-error";
import type { LoggedInUser } from "@/lib/hooks/auth/use-current-user";
import { api } from "@/lib/utils/axios-api";
import { createFileRoute, Outlet, redirect } from "@tanstack/solid-router";

export const Route = createFileRoute("/__authenticated")({
	component: RouteComponent,
	beforeLoad: async ({ location }) => {
		try {
			const response = await api.get<LoggedInUser>("/v1/auth/current-user");
			return response.data;
		} catch (e) {
			console.log("error", e);
			if (e instanceof ApiError && e.status === 401) {
				if (e.message === "password_reset_required") {
					throw redirect({
						to: "/auth/reset-password",
						search: {
							redirect: location.pathname,
							redirectSearch: location.search,
						},
					});
				}
					throw redirect({
						to: "/auth/login",
						search: {
							redirect: location.pathname,
							redirectSearch: location.search,
						},
					});
			}
		}
	},
});

function RouteComponent() {
	return (
		<>
			<Nav />
			<Outlet />
		</>
	);
}
