import { Router } from "@solidjs/router";
import { FileRoutes } from "@solidjs/start/router";
import { Show } from "solid-js";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import { SolidQueryDevtools } from "@tanstack/solid-query-devtools";
import { MetaProvider } from "@solidjs/meta";
import { Layout } from "./components/layout";

import "./app.css";

const queryClient = new QueryClient();

export default function App() {
	return (
		<QueryClientProvider client={queryClient}>
			<MetaProvider>
				<Router root={Layout}>
					<FileRoutes />
				</Router>
			</MetaProvider>
			<Show when={import.meta.env.DEV}>
				<SolidQueryDevtools buttonPosition="top-right" />
			</Show>
		</QueryClientProvider>
	);
}
