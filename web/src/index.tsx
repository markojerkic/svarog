/* @refresh reload */
import { render } from "solid-js/web";
import "./app.css";
import App from "./app.tsx";
import type { ApiError } from "@/lib/api-error.ts";

declare module "@tanstack/solid-query" {
	interface Register {
		defaultError: ApiError;
	}
}

const root = document.getElementById("root");

if (!root) {
	throw Error("No root element");
}

render(() => <App />, root);
