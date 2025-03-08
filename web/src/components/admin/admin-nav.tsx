import { cn } from "@/lib/cn";
import { buttonVariants } from "@/components/ui/button";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import { For, type JSX, Show } from "solid-js";
import { A, useMatch } from "@solidjs/router";

export type NavListItem = {
	title: string;
	label?: string;
	icon: JSX.Element;
	href: string;
};
type Props = {
	isCollapsed: boolean;
	links: NavListItem[];
};

export const NavListItems = (props: Props) => {
	return (
		<div
			data-collapsed={props.isCollapsed}
			class="group flex flex-col gap-4 py-2 data-[collapsed=true]:py-2"
		>
			<nav class="grid gap-1 px-2 group-data-[collapsed=true]:justify-center group-data-[collapsed=true]:px-2">
				<For each={props.links}>
					{(item) => (
						<NavListItem item={item} isCollapsed={props.isCollapsed} />
					)}
				</For>
			</nav>
		</div>
	);
};

const NavListItem = (props: { item: NavListItem; isCollapsed: boolean }) => {
	const match = useMatch(() => props.item.href);
	const variant = () => (match() ? "default" : "ghost");

	return (
		<Show
			when={props.isCollapsed}
			fallback={
				<A
					href={props.item.href}
					class={cn(
						buttonVariants({
							variant: variant(),
							size: "sm",
							class: "text-sm",
						}),
						variant() === "default" &&
							"dark:bg-muted dark:text-white dark:hover:bg-muted dark:hover:text-white",
						"justify-start",
					)}
				>
					<div class="mr-2">{props.item.icon}</div>
					{props.item.title}
					{props.item.label && (
						<span
							class={cn(
								"ml-auto",
								variant() === "default" && "text-background dark:text-white",
							)}
						>
							{props.item.label}
						</span>
					)}
				</A>
			}
		>
			<Tooltip openDelay={0} closeDelay={0} placement="right">
				<TooltipTrigger
					as="a"
					href={props.item.href}
					class={cn(
						buttonVariants({ variant: variant(), size: "icon" }),
						"h-9 w-9",
						variant() === "default" &&
							"dark:bg-muted dark:text-muted-foreground dark:hover:bg-muted dark:hover:text-white",
					)}
				>
					{props.item.icon}
					<span class="sr-only">{props.item.title}</span>
				</TooltipTrigger>
				<TooltipContent class="flex items-center gap-4">
					{props.item.title}
					<Show when={props.item.label}>
						<span class="ml-auto text-muted-foreground">
							{props.item.label}
						</span>
					</Show>
				</TooltipContent>
			</Tooltip>
		</Show>
	);
};
