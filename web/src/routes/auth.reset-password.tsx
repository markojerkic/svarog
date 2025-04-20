import { createFileRoute } from "@tanstack/solid-router";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { TextFormField } from "@/components/ui/textfield";
import {
	type ResetPasswordInput,
	resetPasswordSchema,
	useResetPassword,
} from "@/lib/hooks/auth/use-reset-password";
import { createForm, setError, valiForm } from "@modular-forms/solid";
import { Show } from "solid-js";
import * as v from "valibot";

const schema = v.object({
	redirect: v.optional(v.pipe(v.string())),
	redirectSearch: v.optional(v.any()),
});

export const Route = createFileRoute("/auth/reset-password")({
	component: RouteComponent,
	validateSearch: schema,
});

function RouteComponent() {
	return (
		<Card class="container w-full md:w-[70%] lg:w-[50%]">
			<CardHeader>
				<CardTitle>Reset password</CardTitle>
			</CardHeader>
			<CardContent>
				<div class="grid gap-2">
					<RestPasswordForm />
				</div>
			</CardContent>
		</Card>
	);
}

const RestPasswordForm = () => {
	const navigate = Route.useNavigate();
	const search = Route.useSearch();

	const [form, { Form, Field }] = createForm<ResetPasswordInput>({
		validate: valiForm(resetPasswordSchema),
	});
	const resetPassword = useResetPassword(form);

	const handleSubmit = (values: ResetPasswordInput) => {
		if (values.password !== values.repeatedPassword) {
			setError(form, "password", "Passwords do not match");
			setError(form, "repeatedPassword", "Passwords do not match");
			return;
		}

		resetPassword.mutate(values, {
			onSuccess: () => {
				if (search().redirect) {
					navigate({
						to: search().redirect,
						search: search().redirectSearch,
					});
					return;
				}
				console.log("redirecting to /");
				navigate({
					to: "/",
				});
			},
		});
	};

	return (
		<Form class="grid grid-cols-1 gap-4" onSubmit={handleSubmit}>
			<Field type="string" name="password">
				{(field, props) => (
					<TextFormField
						{...props}
						type="password"
						label="Password"
						error={field.error}
						value={field.value as string | undefined}
						required
					/>
				)}
			</Field>
			<Field type="string" name="repeatedPassword">
				{(field, props) => (
					<TextFormField
						{...props}
						type="password"
						label="Repeated password"
						error={field.error}
						value={field.value as string | undefined}
						required
					/>
				)}
			</Field>

			<Show when={resetPassword.isError}>
				<p class="py-2 text-red-500">{resetPassword.error?.message}</p>
			</Show>

			<Button
				type="submit"
				disabled={resetPassword.isPending || form.submitting}
			>
				ResetPassword
			</Button>
		</Form>
	);
};
