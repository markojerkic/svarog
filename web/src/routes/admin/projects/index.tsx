import { Button } from "@/components/ui/button";
import { api } from "@/lib/utils/axios-api";
import type { RouteDefinition } from "@solidjs/router";
import { createMutation } from "@tanstack/solid-query";
import { Suspense } from "solid-js";
import { toast } from "solid-sonner";

export const projectsRoute = {} satisfies RouteDefinition;

export default () => {
	const generate = createMutation(() => ({
		mutationKey: ["generate-ca-cert"],
		mutationFn: async () => {
			return api.post("/v1/certificate/generate-ca").then((res) => res.data);
		},
		onSuccess: () => toast.success("Certificate generated"),
		onError: (err) => toast.error(err.message),
	}));

	return (
		<div>
			<p>We're gonna generate some certificates</p>
			<Button disabled={generate.isPending} onClick={() => generate.mutate()}>
				Generate
			</Button>

			<Suspense fallback="Loading...">
				<pre>{JSON.stringify(generate.data)}</pre>
			</Suspense>
		</div>
	);
};
