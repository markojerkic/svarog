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

//<nav class="bg-sky-800">
//	<ul class="container flex justify-between p-3 text-gray-200">
//		<li class={`border-b-2 ${active("/")} mx-1.5 sm:mx-6`}>
//			<a href="/">Home</a>
//		</li>
//
//		{/**user icon*/}
//		<li>
//			<UserIcon />
//		</li>
//	</ul>
//</nav>
