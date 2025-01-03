import { Router } from "@solidjs/router";
import { Show } from "solid-js";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import { SolidQueryDevtools } from "@tanstack/solid-query-devtools";
import { MetaProvider } from "@solidjs/meta";
import { Layout } from "./components/layout";
import routes from "./routes";
import { NotLoggedInError } from "@/lib/errors/not-logged-in-error";

export default function App() {
	const queryClient = new QueryClient({
		defaultOptions: {
			queries: {
				refetchOnWindowFocus: true,
				throwOnError: true,
				retry: (faliureCount, error) => {
					if (error instanceof NotLoggedInError) {
						return false;
					}

					return !import.meta.env.DEV && faliureCount < 3;
				},
			},
			mutations: {
				retry: false,
				throwOnError: false,
			},
		},
	});

	return (
		<QueryClientProvider client={queryClient}>
			<MetaProvider>
				<Router root={Layout}>{routes}</Router>
			</MetaProvider>
			<Show when={import.meta.env.DEV}>
				<SolidQueryDevtools buttonPosition="top-right" />
			</Show>
		</QueryClientProvider>
	);
}
