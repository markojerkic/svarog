import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import type { DialogTriggerProps } from "@kobalte/core/dialog";
import { createForm, valiForm } from "@modular-forms/solid";
import { createSignal, Show } from "solid-js";
import { toast } from "solid-sonner";
import Eraser from "lucide-solid/icons/eraser";
import {
	DropdownMenuItem,
	DropdownMenuItemLabel,
} from "@/components/ui/dropdown-menu";
import {
	type AddClientInput,
	addClientSchema,
	useAddClient,
} from "@/lib/hooks/projects/add-remove-client";
import type { Project } from "@/lib/hooks/projects/use-projects";
import { FormSelect } from "@/components/ui/select";

export const RemoveClient = (props: { project: Project }) => {
	const [open, setOpen] = createSignal(false);

	return (
		<Dialog open={open()} onOpenChange={setOpen}>
			<DialogTrigger
				as={(props: DialogTriggerProps) => (
					<DropdownMenuItem {...props} closeOnSelect={false}>
						<Eraser class="h-5" />
						<DropdownMenuItemLabel>Remove client</DropdownMenuItemLabel>
					</DropdownMenuItem>
				)}
			/>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Remove client from project</DialogTitle>
					<DialogDescription>
						Select a client to remove from the project.
					</DialogDescription>
				</DialogHeader>
				<RemoveClientForm
					project={props.project}
					onSuccess={() => setOpen(false)}
				/>
			</DialogContent>
		</Dialog>
	);
};

const RemoveClientForm = (props: {
	project: Project;
	onSuccess: () => void;
}) => {
	const [form, { Form, Field }] = createForm<AddClientInput>({
		validate: valiForm(addClientSchema),
		initialValues: { projectId: props.project.id },
	});
	const createClient = useAddClient(form);

	const clientOptions = () =>
		props.project.clients.map((client) => ({ label: client, value: client }));

	const handleSubmit = async (values: AddClientInput) => {
		createClient.mutateAsync(values).then(() => {
			props.onSuccess();
			toast.success("Client created");
		});
	};

	return (
		<Form class="flex flex-col gap-4" onSubmit={handleSubmit}>
			<Field type="string" name="projectId">
				{(field, props) => (
					<input type="hidden" {...props} value={field.value} />
				)}
			</Field>

			<Field type="string" name="clientName">
				{(field, formItemProps) => (
					<FormSelect
						{...formItemProps}
						name="clientName"
						placeholder="Select a client"
						options={clientOptions()}
						value={field.value}
						error={field.error}
						required
						ref={formItemProps.ref}
						onInput={formItemProps.onInput}
						onChange={formItemProps.onChange}
						onBlur={formItemProps.onBlur}
					/>
				)}
			</Field>

			<Show when={createClient.isError}>
				<p class="py-2 text-red-500">{createClient.error?.message}</p>
			</Show>

			<Button
				variant="destructive"
				type="submit"
				disabled={createClient.isPending || form.submitting}
			>
				Delete
			</Button>
		</Form>
	);
};
