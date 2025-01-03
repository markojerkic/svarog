import {
	Card,
	CardContent,
	CardDescription,
	CardFooter,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import {
	TextField,
	TextFieldLabel,
	TextFieldRoot,
} from "@/components/ui/textfield";

export default () => {
	return (
		<Card class="container mt-6 max-w-fit">
			<CardHeader>
				<CardTitle>Card Title</CardTitle>
				<CardDescription>Card Description</CardDescription>
			</CardHeader>
			<CardContent>
				<div class="grid gap-2">
					<TextFieldRoot>
						<TextFieldLabel>Email</TextFieldLabel>
						<TextField type="email" placeholder="m@example.com" />
					</TextFieldRoot>
				</div>

				<div class="grid gap-2">
					<TextFieldRoot>
						<TextFieldLabel>Password</TextFieldLabel>
						<TextField type="Password" />
					</TextFieldRoot>
				</div>
			</CardContent>
			<CardFooter>
				<p>Card Footer</p>
			</CardFooter>
		</Card>
	);
};
