import { AdminLayout } from "@/components/admin/layout";
import {
	currentUserQueryOptions,
	useCurrentUser,
} from "@/lib/hooks/auth/use-current-user";
import type { RouteDefinition } from "@solidjs/router";
import { useQueryClient } from "@tanstack/solid-query";
import { ParentProps, Show, Suspense, lazy } from "solid-js";
import { usersRoute } from "./users";

export const adminRoute = {
	preload: async () => {
		return useQueryClient().prefetchQuery(currentUserQueryOptions);
	},
	children: [
		{
			path: "/users",
			...usersRoute,
			component: lazy(() => import("./users")),
		},
	],
} satisfies RouteDefinition;

export default (props: ParentProps) => {
	const currentUser = useCurrentUser();

	const isAdmin = () => currentUser.data?.role === "admin";

	return (
		<Suspense>
			<Show
				when={currentUser.isSuccess && isAdmin()}
				fallback={<div>Nisi admin, nemaš pravo ovo gledati </div>}
			>
				<AdminLayout>{props.children}</AdminLayout>
			</Show>
		</Suspense>
	);
};
