import { AdminLayout } from "@/components/admin/layout";
import {
	currentUserQueryOptions,
	useCurrentUser,
} from "@/lib/hooks/auth/use-current-user";
import type { RouteDefinition } from "@solidjs/router";
import { useQueryClient } from "@tanstack/solid-query";
import { Show, Suspense } from "solid-js";

export const adminRoute = {
	preload: async () => {
		return useQueryClient().prefetchQuery(currentUserQueryOptions);
	},
} satisfies RouteDefinition;

export default () => {
	const currentUser = useCurrentUser();

	const isAdmin = () => currentUser.data?.role === "admin";

	return (
		<Suspense>
			<Show
				when={currentUser.isSuccess && isAdmin()}
				fallback={<div>Nisi admin, nemaš pravo ovo gledati </div>}
			>
				<AdminLayout>Brabo, admin si</AdminLayout>
			</Show>
		</Suspense>
	);
};
