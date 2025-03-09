/* @refresh reload */
import { render } from "solid-js/web";
import { RouterProvider, createRouter } from "@tanstack/solid-router";
import { routeTree } from "./routeTree.gen";
import "./app.css";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import { SolidQueryDevtools } from "@tanstack/solid-query-devtools";
import { ApiError } from "./lib/errors/api-error";
import { Toaster } from "solid-sonner";
import { Show } from "solid-js";
import { useCurrentUser } from "./lib/hooks/auth/use-current-user";

const queryClient = new QueryClient({
	defaultOptions: {
		queries: {
			refetchOnWindowFocus: true,
			throwOnError: true,
			retry: (faliureCount, error) => {
				if (error instanceof ApiError) {
					return error.status !== 401 && error.status !== 403;
				}

				return !import.meta.env.DEV && faliureCount < 3;
			},
		},
		mutations: {
			retry: false,
		},
	},
});

const router = createRouter({
	routeTree,
	defaultPreload: "intent",
	scrollRestoration: true,
	defaultStaleTime: 1000,
	context: {
		queryClient,
	},
});

declare module "@tanstack/solid-router" {
	interface Register {
		router: typeof router;
	}
}

declare module "@tanstack/solid-query" {
	interface Register {
		defaultError: ApiError;
	}
}

const root = document.getElementById("root");

if (!root) {
	throw Error("No root element");
}

const AuthProvider = () => {
	const currentUser = useCurrentUser();

	return (
		<Show when={!currentUser.isPending}>
			<RouterProvider
				router={router}
				context={{
					auth: currentUser.data,
					authError: currentUser.error,
					queryClient,
				}}
			/>
		</Show>
	);
};

const App = () => {
	return (
		<QueryClientProvider client={queryClient}>
			<AuthProvider />
			<Toaster />
			<SolidQueryDevtools />
		</QueryClientProvider>
	);
};

render(App, root);
