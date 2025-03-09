import { Nav } from "@/components/navigation/nav";
import type { LoggedInUser } from "@/lib/hooks/auth/use-current-user";
import { api } from "@/lib/utils/axios-api";
import { createFileRoute, Outlet, redirect } from "@tanstack/solid-router";
import { AxiosError } from "axios";

export const Route = createFileRoute("/__authenticated")({
	component: RouteComponent,
	beforeLoad: async ({ location }) => {
		try {
			const response = await api.get<LoggedInUser>("/v1/auth/current-user");
			return response.data;
		} catch (e) {
			if (e instanceof AxiosError && e.response?.status === 401) {
				if (e.message === "password_reset_required") {
					throw redirect({
						to: "/",
						search: {
							redirect: location.href,
						},
					});
				}
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
