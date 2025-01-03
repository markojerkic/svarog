import {
	Card,
	CardContent,
	CardFooter,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import {
	TextField,
	TextFieldLabel,
	TextFieldRoot,
} from "@/components/ui/textfield";
import { createForm } from "@modular-forms/solid";

export default () => {
	return (
		<Card class="container w-full md:w-[70%] lg:w-[50%]">
			<CardHeader>
				<CardTitle>Login</CardTitle>
			</CardHeader>
			<CardContent>
				<div class="grid gap-2">
					<TextFieldRoot>
						<TextFieldLabel>Email</TextFieldLabel>
						<TextField type="email" placeholder="m@example.com" />
					</TextFieldRoot>
				</div>

				<div class="grid gap-2">
					<LoginForm />
				</div>
			</CardContent>
			<CardFooter>
				<p>Card Footer</p>
			</CardFooter>
		</Card>
	);
};

const LoginForm = () => {
	const [, { Form, Field }] = createForm();

	return (
		<Form>
			<Field type="string" name="email">
				{(_, props) => (
					<TextFieldRoot>
						<TextFieldLabel>Email</TextFieldLabel>
						<TextField {...props} type="email" />
					</TextFieldRoot>
				)}
			</Field>
			<Field type="string" name="password">
				{(_, props) => (
					<TextFieldRoot>
						<TextFieldLabel>Password</TextFieldLabel>
						<TextField {...props} type="password" />
					</TextFieldRoot>
				)}
			</Field>
		</Form>
	);
};
