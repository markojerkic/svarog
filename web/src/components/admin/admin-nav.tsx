import { cn } from "@/lib/cn";
import { buttonVariants } from "@/components/ui/button";
import { For, type JSX } from "solid-js";
import { Link, type LinkComponentProps } from "@tanstack/solid-router";

export type NavListItem = {
	title: string;
	label?: string;
	icon: JSX.Element;
} & LinkComponentProps;
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
	return (
		<Link
			to={props.item.to}
			inactiveProps={{
				class: cn(
					buttonVariants({
						variant: "ghost",
						size: "sm",
						class: "text-sm",
					}),
					"justify-start",
				),
			}}
			link
			activeProps={{
				class: cn(
					buttonVariants({
						variant: "default",
						size: "sm",
						class: "text-sm",
					}),
					"dark:bg-muted dark:text-white dark:hover:bg-muted dark:hover:text-white",
					"justify-start",
				),
			}}
		>
			{(linkProps) => (
				<>
					<div class="mr-2">{props.item.icon}</div>
					{props.item.title}
					{props.item.label && (
						<span
							class={cn(
								"ml-auto",
								linkProps.isActive && "text-background dark:text-white",
							)}
						>
							{props.item.label}
						</span>
					)}
				</>
			)}
		</Link>
	);
};
