import { Suspense } from "solid-js";
import { useCurrentUser } from "~/lib/hooks/auth/use-current-user";

export const UserIcon = () => {
	const currentUser = useCurrentUser();

	return (
		<Suspense fallback="Loading...">
			<div>
				{currentUser.data ? (
					<div>
						<img
							src={`https://avatars.dicebear.com/api/avataaars/${currentUser.data.username}.svg`}
							alt="User avatar"
							class="h-8 w-8 rounded-full"
						/>
						<span class="ml-2">{currentUser.data.username}</span>
					</div>
				) : (
					<div class="animate-bounce text-white">Loading...</div>
				)}
			</div>
		</Suspense>
	);
};
