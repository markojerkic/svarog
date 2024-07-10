// @refresh reload
import { mount, StartClient } from "@solidjs/start/client";

// biome-ignore lint/style/noNonNullAssertion: Needed
mount(() => <StartClient />, document.getElementById("app")!);
