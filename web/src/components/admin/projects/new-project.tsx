import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import {
	type NewProjectInput,
	newProjectSchema,
	useCreateProject,
} from "@/lib/hooks/projects/create-project";
import type { DialogTriggerProps } from "@kobalte/core/dialog";
import { createForm, insert, remove, valiForm } from "@modular-forms/solid";
import {
	RemovableTextFormField,
	TextFormField,
} from "@/components/ui/textfield";
import { createSignal, For, Show } from "solid-js";
import { toast } from "solid-sonner";
import { useRouter } from "@tanstack/solid-router";

export const NewProject = () => {
	const [open, setOpen] = createSignal(false);
	const router = useRouter();

	return (
		<Dialog open={open()} onOpenChange={setOpen}>
			<DialogTrigger
				as={(props: DialogTriggerProps) => (
					<Button variant="outline" {...props}>
						Create new project
					</Button>
				)}
			/>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Create a new project</DialogTitle>
					<DialogDescription>
						Enter the name of the new project
					</DialogDescription>
				</DialogHeader>
				<NewProjectForm
					onSuccess={() => {
						router.invalidate();
					}}
				/>
			</DialogContent>
		</Dialog>
	);
};

const NewProjectForm = (props: { onSuccess: () => void }) => {
	const [form, { Form, Field, FieldArray }] = createForm<NewProjectInput>({
		validate: valiForm(newProjectSchema),
	});
	const createProject = useCreateProject(form);

	const handleSubmit = async (values: NewProjectInput) => {
		console.log("submit", values);
		createProject.mutateAsync(values).then(() => {
			props.onSuccess();
			toast.success("Project created");
		});
	};

	return (
		<Form class="flex flex-col gap-4" onSubmit={handleSubmit}>
			<Field type="string" name="name">
				{(field, props) => (
					<TextFormField
						{...props}
						type="text"
						label="Project name"
						error={field.error}
						value={field.value as string | undefined}
						required
					/>
				)}
			</Field>
			<FieldArray name="clients">
				{(fieldArray) => (
					<>
						<For each={fieldArray.items}>
							{(_, index) => (
								<Field name={`clients.${index()}`}>
									{(field, props) => (
										<RemovableTextFormField
											{...props}
											class="flex-1"
											type="text"
											label="Client name"
											error={field.error}
											value={field.value as string | undefined}
											required
											onRemove={() => remove(form, "clients", { at: index() })}
										/>
									)}
								</Field>
							)}
						</For>
						<Button
							class="self-end"
							onClick={() => {
								insert(form, "clients", { value: "" });
							}}
						>
							Add client
						</Button>
					</>
				)}
			</FieldArray>

			<Show when={createProject.isError}>
				<p class="py-2 text-red-500">{createProject.error?.message}</p>
			</Show>

			<Button
				type="submit"
				disabled={createProject.isPending || form.submitting}
			>
				Submit
			</Button>
		</Form>
	);
};
