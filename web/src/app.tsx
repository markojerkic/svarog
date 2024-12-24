import { Router } from "@solidjs/router";
import { Show } from "solid-js";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import { SolidQueryDevtools } from "@tanstack/solid-query-devtools";
import { MetaProvider } from "@solidjs/meta";
import { Layout } from "./components/layout";
//import routes from "./routes";
import { routes } from "virtual:solid-routes";

export default function App() {
	const queryClient = new QueryClient({
		defaultOptions: {
			queries: {
				refetchOnWindowFocus: true,
			},
		},
	});
	//const _FSRoutes = useRoutes(routes);

	return (
		<QueryClientProvider client={queryClient}>
			<pre>{JSON.stringify(routes, null, 2)}</pre>
			<MetaProvider>
				<Router root={Layout}>{routes}</Router>
			</MetaProvider>
			<Show when={import.meta.env.DEV}>
				<SolidQueryDevtools buttonPosition="top-right" />
			</Show>
		</QueryClientProvider>
	);
}
