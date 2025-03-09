import type { User } from "@/lib/hooks/auth/users";
import { api } from "@/lib/utils/axios-api";
import { createFileRoute } from "@tanstack/solid-router";
import { NewUserDialog } from "@/components/admin/auth/new-user";
import { UserListItem } from "@/components/admin/auth/user-list-item";
import {
	Pagination,
	PaginationEllipsis,
	PaginationItem,
	PaginationItems,
	PaginationNext,
	PaginationPrevious,
} from "@/components/ui/pagination";
import { For } from "solid-js";
import * as v from "valibot";

const schema = v.object({
	page: v.optional(v.number(), 0),
});

export const Route = createFileRoute("/__authenticated/admin/users")({
	component: RouteComponent,
	validateSearch: schema,
	loaderDeps: ({ search }) => search,
	loader: async ({ deps }) => {
		const response = await api.get<User[]>("/v1/auth/users", {
			params: {
				page: deps.page,
			},
		});
		return response.data;
	},
});

function RouteComponent() {
	const users = Route.useLoaderData();

	return (
		<div class="mx-auto w-full p-4 text-center text-gray-700 md:w-[70%] lg:w-[50%]">
			<div class="flex justify-end">
				<NewUserDialog />
			</div>
			<For
				each={users()}
				fallback={<div class="animate-bounce text-white">Loading...</div>}
			>
				{(user) => <UserListItem user={user} />}
			</For>
			<Pagination
				count={10}
				itemComponent={(props) => (
					<PaginationItem page={props.page}>{props.page}</PaginationItem>
				)}
				ellipsisComponent={() => <PaginationEllipsis />}
			>
				<PaginationPrevious />
				<PaginationItems />
				<PaginationNext />
			</Pagination>
		</div>
	);
}
