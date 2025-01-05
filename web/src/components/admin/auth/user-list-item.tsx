import type { User } from "@/lib/hooks/auth/users";

export const UserListItem = (props: { user: User }) => {
	return (
		<div class="flex text-start">
			<div class="flex-1 flex-col gap-2">
				<div class="font-semibold text-lg">{props.user.username}</div>
				<div class="text-sm">{props.user.id}</div>
			</div>
			<div>{props.user.role}</div>
		</div>
	);
};
