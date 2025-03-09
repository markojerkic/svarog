import {
	NavigationMenu,
	NavigationMenuContent,
	NavigationMenuItem,
	NavigationMenuTrigger,
} from "@/components/ui/navigation-menu";
import type { LoggedInUser } from "@/lib/hooks/auth/use-current-user";
import { Link } from "@tanstack/solid-router";
import { type ParentProps, Show, Suspense } from "solid-js";

export function Nav(props: { currentUser: LoggedInUser }) {
	return (
		<NavigationMenu class="w-full gap-3 border-b border-b-secondary p-3">
			<NavigationMenuItem>
				<NavigationMenuTrigger withArrow={false}>
					<Link to="/">Home</Link>
				</NavigationMenuTrigger>
			</NavigationMenuItem>
			<Suspense>
				<NavigationMenuItem>
					<AuthMenuItem user={props.currentUser} />
				</NavigationMenuItem>
			</Suspense>
		</NavigationMenu>
	);
}

const AuthMenuItem = (props: { user: LoggedInUser }) => {
	return (
		<>
			<NavigationMenuTrigger>Settings</NavigationMenuTrigger>
			<NavigationMenuContent class="grid w-[400px] gap-3 p-4 lg:w-[500px] lg:grid-cols-[.75fr_1fr] [&>li:first-of-type]:row-span-3">
				<ListItem title="Profile" href="/auth/profile">
					{props.user.username}
				</ListItem>
				<Show when={props.user.role === "admin"}>
					<ListItem title="Admin" href="/admin">
						Manage users and roles
					</ListItem>
				</Show>

				<ListItem title="Logout" href="/auth/logout" />
			</NavigationMenuContent>
		</>
	);
};

const ListItem = (props: ParentProps<{ title: string; href: string }>) => {
	return (
		<NavigationMenuItem>
			<Link
				to={props.href}
				class="block select-none space-y-1 rounded-md p-3 leading-none no-underline outline-hidden transition-[box-shadow,background-color] duration-200 hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground focus-visible:outline-hidden focus-visible:ring-[1.5px] focus-visible:ring-ring"
			>
				<span class="font-medium text-sm leading-none">{props.title}</span>
				<span class="line-clamp-2 text-muted-foreground text-sm leading-snug">
					{props.children}
				</span>
			</Link>
		</NavigationMenuItem>
	);
};
