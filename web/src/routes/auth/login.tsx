import { Button } from "@/components/ui/button";
import {
    Card,
    CardContent,
    CardFooter,
    CardHeader,
    CardTitle,
} from "@/components/ui/card";
import { TextFormField } from "@/components/ui/textfield";
import {
    type LoginInput,
    loginSchema,
    useLogin,
} from "@/lib/hooks/auth/login-register";
import { createForm, setError, valiForm } from "@modular-forms/solid";
import { createEffect } from "solid-js";

export default () => {
    return (
        <Card class="container w-full md:w-[70%] lg:w-[50%]">
            <CardHeader>
                <CardTitle>Login</CardTitle>
            </CardHeader>
            <CardContent>
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
    const [form, { Form, Field }] = createForm<LoginInput>({
        validate: valiForm(loginSchema),
    });

    const login = useLogin(form);

    const handleSubmit = async (values: LoginInput) => {
        login.mutate(values);
    };

    return (
        <Form onSubmit={handleSubmit}>
            <Field type="string" name="email">
                {(field, props) => (
                    <TextFormField
                        {...props}
                        type="email"
                        label="Email"
                        error={login.error?.message ?? field.error}
                        value={field.value as string | undefined}
                        required
                    />
                )}
            </Field>
            <Field type="string" name="password">
                {(field, props) => (
                    <TextFormField
                        {...props}
                        type="password"
                        label="Password"
                        error={login.error?.message ?? field.error}
                        value={field.value as string | undefined}
                        required
                    />
                )}
            </Field>

            <Button type="submit" disabled={login.isPending}>
                Login
            </Button>
        </Form>
    );
};
