import { useMatch, type RouteSectionProps } from "@solidjs/router";
import { Suspense } from "solid-js";
import { Nav } from "~/components/nav";

export const Layout = (props: RouteSectionProps<unknown>) => {
    const isLogsRoute = useMatch(() => "/logs/:clientId");

    return (
        <div class="flex flex-col justify-start"
            classList={{
                "h-screen": Boolean(isLogsRoute())
            }}
        >
            <Nav />
            <Suspense>{props.children}</Suspense>
        </div>
    );
};
