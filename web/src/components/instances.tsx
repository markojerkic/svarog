import { For } from "solid-js";

export const Instances = (props: { instances: string[] }) => {
	return (
		<nav class="inline-flex bg-red-200 p-2">
			<For each={props.instances}>
				{(instance) => (
					<button
						type="button"
						onClick={() => alert(`${instance}: Not implemented`)}
					>
						{instance}
					</button>
				)}
			</For>
		</nav>
	);
};
