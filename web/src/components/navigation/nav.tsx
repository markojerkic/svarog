import {
    NavigationMenu,
    NavigationMenuItem,
    NavigationMenuLink,
} from "@/components/ui/navigation-menu";
import { useCurrentUser } from "@/lib/hooks/auth/use-current-user";
import { Match, Suspense, Switch } from "solid-js";

export function Nav() {
    const currentUser = useCurrentUser();

    //const active = (path: string) =>
    //    path === location.pathname
    //        ? "border-sky-600"
    //        : "border-transparent hover:border-sky-600";

    return (
        <NavigationMenu class="w-full gap-3 bg-accent p-3">
            <NavigationMenuItem>
                <NavigationMenuLink href="/">Home</NavigationMenuLink>
            </NavigationMenuItem>
            <Suspense>
                <NavigationMenuItem>
                    <Switch>
                        <Match when={currentUser.isSuccess}>
                            <pre>{JSON.stringify(currentUser.data)}</pre>
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
