import type { Project } from "@/lib/hooks/projects/use-projects";
import {
	Table,
	TableBody,
	TableCaption,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { createSignal, For } from "solid-js";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuGroupLabel,
	DropdownMenuItem,
	DropdownMenuItemLabel,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { DropdownMenuSubTriggerProps } from "@kobalte/core/dropdown-menu";
import EllipsisVertical from "lucide-solid/icons/ellipsis-vertical";
import Pencil from "lucide-solid/icons/pencil";
import Minus from "lucide-solid/icons/minus";
import HardDriveDownload from "lucide-solid/icons/hard-drive-download";
import {
	AlertDialog,
	AlertDialogClose,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
	AlertDialogAction,
	AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { useDeleteProject } from "@/lib/hooks/projects/delete-project";
import { toast } from "solid-sonner";
import type {
	AlertDialogCloseButtonProps,
	AlertDialogTriggerProps,
} from "@kobalte/core/alert-dialog";
import { AddClient } from "./add-client";
import { RemoveClient } from "./remove-client";

export const ProjectList = (props: { projects: Project[] }) => {
	return (
		<Table>
			<TableCaption>A list of your projects.</TableCaption>
			<TableHeader>
				<TableRow>
					<TableHead>Project name</TableHead>
					<TableHead>Clients</TableHead>
					<TableHead class="w-[100px]" />
				</TableRow>
			</TableHeader>
			<TableBody>
				<For each={props.projects}>
					{(project) => (
						<TableRow>
							<TableCell class="font-medium">{project.name}</TableCell>
							<TableCell>
								<Clients clients={project.clients} />
							</TableCell>
							<TableCell>
								<ProjectActions project={project} />
							</TableCell>
						</TableRow>
					)}
				</For>
			</TableBody>
		</Table>
	);
};

const Clients = (props: { clients: string[] }) => {
	const clients = () => (props.clients ?? []).join(", ");

	return <span>{clients()}</span>;
};

const ProjectActions = (props: { project: Project }) => {
	return (
		<DropdownMenu>
			<DropdownMenuTrigger
				as={(props: DropdownMenuSubTriggerProps) => (
					<Button variant="outline" {...props}>
						<EllipsisVertical />
					</Button>
				)}
			/>
			<DropdownMenuContent class="font-light">
				<DropdownMenuItem>
					<Pencil class="h-5" />
					<DropdownMenuItemLabel>Edit</DropdownMenuItemLabel>
				</DropdownMenuItem>
				<DeleteProjectDialog projectId={props.project.id} />
				<DownloadCertificatesMenuItem projectId={props.project.id} />

				<DropdownMenuSeparator />
				<DropdownMenuGroup>
					<DropdownMenuGroupLabel>Clients</DropdownMenuGroupLabel>
					<AddClient projectId={props.project.id} />
					<RemoveClient project={props.project} />
				</DropdownMenuGroup>
			</DropdownMenuContent>
		</DropdownMenu>
	);
};

const DeleteProjectDialog = (props: { projectId: string }) => {
	const [openDelteProjectDialog, setOpenDelteProjectDialog] =
		createSignal(false);
	const deleteProject = useDeleteProject();

	const handleDeleteProject = async () => {
		deleteProject.mutateAsync(props.projectId).then(() => {
			toast.success("Project deleted");
			setOpenDelteProjectDialog(false);
		});
	};

	return (
		<AlertDialog
			open={openDelteProjectDialog()}
			onOpenChange={setOpenDelteProjectDialog}
		>
			<AlertDialogTrigger
				as={(props: AlertDialogTriggerProps) => (
					<DropdownMenuItem {...props} closeOnSelect={false}>
						<Minus class="h-5" />
						<DropdownMenuItemLabel>Delete project</DropdownMenuItemLabel>
					</DropdownMenuItem>
				)}
			/>

			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>Are you absolutely sure?</AlertDialogTitle>
					<AlertDialogDescription>
						This action cannot be undone. This will permanently delete your
						project and clients associated with it.
					</AlertDialogDescription>
				</AlertDialogHeader>
				<AlertDialogFooter>
					<AlertDialogClose>Cancel</AlertDialogClose>
					<AlertDialogAction
						as={(props: AlertDialogCloseButtonProps) => (
							<Button
								{...props}
								variant="destructive"
								disabled={deleteProject.isPending}
								onClick={() => handleDeleteProject()}
							>
								Delete
							</Button>
						)}
					/>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
};

const DownloadCertificatesMenuItem = (props: { projectId: string }) => {
	return (
		<DropdownMenuItem
			as="a"
			href={`${import.meta.env.VITE_API_URL}/v1/projects/${props.projectId}/certificate`}
			target="_blank"
		>
			<HardDriveDownload class="h-5" />
			<DropdownMenuItemLabel>Download certificates</DropdownMenuItemLabel>
		</DropdownMenuItem>
	);
};
