// virtual-routes.d.ts

declare module "virtual:solid-routes" {
	import type { Component } from "solid-js";
	import type { RouteDefinition } from "@solidjs/router";

	export interface RouteConfig {
		preload?: (params: Record<string, string>) => Promise<unknown>;
		// Add any other custom route properties you want to support
		title?: string;
		middleware?: string[];
		roleRequired?: string;
	}

	export type Route = RouteDefinition &
		RouteConfig & {
			component: Component;
			path: string;
			children?: Route[];
		};

	export const routes: Route[];
}
