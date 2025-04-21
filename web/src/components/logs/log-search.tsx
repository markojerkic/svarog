import { createEffect, createSignal, onCleanup } from "solid-js";
import { TextField, TextFieldRoot } from "../ui/textfield";

const SEARCH_KEY = "f";

export const SearchCommnad = (props: {
	search?: string;
	onInput: (search: string) => void;
}) => {
	let searchInputRef: HTMLInputElement | null = null;
	const [searchQuery, setSearchQuery] = createSignal(props.search);

	createEffect(() => {
		const down = (e: KeyboardEvent) => {
			if (e.key === SEARCH_KEY.toLowerCase() && (e.metaKey || e.ctrlKey)) {
				e.preventDefault();
				if (searchInputRef) {
					searchInputRef.focus();
					searchInputRef.select();
				}
			}
		};

		document.addEventListener("keydown", down);

		onCleanup(() => {
			document.removeEventListener("keydown", down);
		});
	});

	return (
		<>
			<form
				class="p-4"
				onSubmit={(e) => {
					e.preventDefault();
					props.onInput(searchQuery() ?? "");
					searchInputRef?.blur();
				}}
			>
				<TextFieldRoot>
					<div class="relative">
						{/* Placeholder always visible */}
						<span class="-translate-y-1/2 pointer-events-none absolute top-1/2 left-4 transform text-gray-400">
							Press ⌘{SEARCH_KEY.toUpperCase()}
						</span>

						{/* Input field */}
						<TextField
							ref={(el) => {
								searchInputRef = el as HTMLInputElement;
							}}
							value={searchQuery()}
							placeholder="search..."
							onInput={(e) =>
								setSearchQuery((e.target as HTMLInputElement).value)
							}
							class="pl-[7em] text-black"
						/>
					</div>
				</TextFieldRoot>
			</form>
		</>
	);
};
