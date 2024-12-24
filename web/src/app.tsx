import { Router } from "@solidjs/router";
import { Show } from "solid-js";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import { SolidQueryDevtools } from "@tanstack/solid-query-devtools";
import { MetaProvider } from "@solidjs/meta";
import { Layout } from "./components/layout";
import routes from "./routes";

export default function App() {
	const queryClient = new QueryClient({
		defaultOptions: {
			queries: {
				refetchOnWindowFocus: true,
			},
		},
	});

	return (
		<QueryClientProvider client={queryClient}>
			<MetaProvider>
				<Layout>
					<Router>{routes}</Router>
				</Layout>
			</MetaProvider>
			<Show when={import.meta.env.DEV}>
				<SolidQueryDevtools buttonPosition="top-right" />
			</Show>
		</QueryClientProvider>
	);
}
