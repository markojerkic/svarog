import {
	NavigationMenu,
	NavigationMenuContent,
	NavigationMenuDescription,
	NavigationMenuItem,
	NavigationMenuItemLabel,
	NavigationMenuLink,
	NavigationMenuTrigger,
} from "@/components/ui/navigation-menu";
import {
	type LoggedInUser,
	useCurrentUser,
} from "@/lib/hooks/auth/use-current-user";
import { Match, type ParentProps, Show, Suspense, Switch } from "solid-js";

export function Nav() {
	const currentUser = useCurrentUser();

	return (
		<NavigationMenu class="w-full gap-3 border-b border-b-secondary p-3">
			<NavigationMenuTrigger
				class="transition-[box-shadow,background-color] focus-visible:outline-none focus-visible:ring-[1.5px] focus-visible:ring-ring data-[expanded]:bg-accent"
				as="a"
				href="/"
			>
				Home
			</NavigationMenuTrigger>
			<Suspense>
				<NavigationMenuItem>
					<Switch>
						<Match when={currentUser.isSuccess && currentUser.data}>
							<AuthMenuItem user={currentUser.data!} />
						</Match>
						<Match when={currentUser.isError}>
							<NavigationMenuLink href="/auth/login">Login</NavigationMenuLink>
						</Match>
					</Switch>
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
		<NavigationMenuLink
			href={props.href}
			class="block select-none space-y-1 rounded-md p-3 leading-none no-underline outline-none transition-[box-shadow,background-color] duration-200 hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground focus-visible:outline-none focus-visible:ring-[1.5px] focus-visible:ring-ring"
		>
			<NavigationMenuItemLabel class="font-medium text-sm leading-none">
				{props.title}
			</NavigationMenuItemLabel>
			<NavigationMenuDescription class="line-clamp-2 text-muted-foreground text-sm leading-snug">
				{props.children}
			</NavigationMenuDescription>
		</NavigationMenuLink>
	);
};
