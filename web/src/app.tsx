import { Router } from "@solidjs/router";
import { FileRoutes } from "@solidjs/start/router";
import { Show, Suspense } from "solid-js";
import Nav from "~/components/Nav";
import "./app.css";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import { SolidQueryDevtools } from "@tanstack/solid-query-devtools";

const queryClient = new QueryClient();

export default function App() {
	return (
		<QueryClientProvider client={queryClient}>
			<Router
				root={(props) => (
					<>
						<Nav />
						<Suspense>{props.children}</Suspense>
					</>
				)}
			>
				<FileRoutes />
			</Router>
			<Show when={import.meta.env.DEV}>
				<SolidQueryDevtools />
			</Show>
		</QueryClientProvider>
	);
}
