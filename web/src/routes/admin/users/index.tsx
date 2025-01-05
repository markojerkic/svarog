import { UserListItem } from "@/components/admin/auth/user-list-item";
import { useUsers } from "@/lib/hooks/auth/users";
import { useParams, type RouteDefinition } from "@solidjs/router";
import { useQueryClient } from "@tanstack/solid-query";
import { Suspense, For } from "solid-js";

export const usersRoute = {
	preload: async () => {
		return useQueryClient().prefetchQuery(useUsers.QUERY_OPTIONS(0));
	},
} satisfies RouteDefinition;

export default () => {
	const parameters = useParams();
	const page = () => (parameters.page ? +parameters.page : 0);
	const users = useUsers(() => page());

	return (
		<Suspense>
			<div class="mx-auto p-4 text-center text-gray-700">
				<p>Total users: {users.data?.length}</p>
				<For
					each={users.data}
					fallback={<div class="animate-bounce text-white">Loading...</div>}
				>
					{(user) => <UserListItem user={user} />}
				</For>
			</div>
		</Suspense>
	);
};
