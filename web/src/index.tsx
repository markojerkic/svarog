/* @refresh reload */
import { render } from "solid-js/web";
import "./app.css";
import App from "./app.tsx";

const root = document.getElementById("root");

if (!root) {
	throw Error("No root element");
}

render(() => <App />, root);
