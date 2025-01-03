import { ErrorBoundary, Suspense } from "solid-js";
import { useCurrentUser } from "@/lib/hooks/auth/use-current-user";

export const UserIcon = () => {
	const currentUser = useCurrentUser();

	return (
		<div>
			<ErrorBoundary
				fallback={<div class="font-bold text-red-500">No user</div>}
			>
				<Suspense
					fallback={<div class="animate-bounce text-white">Loading...</div>}
				>
					<div>
						<span class="ml-2">{currentUser.data?.username}</span>
					</div>
				</Suspense>
			</ErrorBoundary>
		</div>
	);
};
