import type { User } from "@/lib/hooks/auth/users";

export const UserListItem = (props: { user: User }) => {
	const name = () => `${props.user.firstName} ${props.user.lastName}`;

	return (
		<div class="flex text-start">
			<div class="flex-1 flex-col gap-2">
				<div class="font-semibold text-lg">{name()}</div>
				<div class="text-sm">{props.user.username}</div>
			</div>
			<div>{props.user.role}</div>
		</div>
	);
};
