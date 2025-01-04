import { useLocation } from "@solidjs/router";
import {
	NavigationMenu,
	NavigationMenuItem,
	NavigationMenuLink,
} from "@/components/ui/navigation-menu";

export function Nav() {
	const location = useLocation();
	const active = (path: string) =>
		path === location.pathname
			? "border-sky-600"
			: "border-transparent hover:border-sky-600";

	return (
		<NavigationMenu class="w-full gap-3 bg-accent p-3">
			<NavigationMenuItem>
				<NavigationMenuLink href="/">Home</NavigationMenuLink>
			</NavigationMenuItem>
			<NavigationMenuItem>
				<NavigationMenuLink href="/auth/login">Login</NavigationMenuLink>
			</NavigationMenuItem>
		</NavigationMenu>
	);
}
