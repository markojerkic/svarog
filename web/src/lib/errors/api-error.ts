import {
	type FieldValues,
	type FormStore,
	setError,
} from "@modular-forms/solid";

type ApiFieldErrors = Record<string, string>;
export type TApiError = { message: string; fields: ApiFieldErrors };

export class ApiError extends Error {
	constructor(
		public readonly apiError: TApiError,
		public readonly status: number,
	) {
		super(apiError.message);
	}

	public setFormFieldErrors<T extends FieldValues>(form: FormStore<T>) {
		for (const [fieldName, message] of Object.entries(this.apiError.fields)) {
			// @ts-expect-error fieldName is typesafe, but this is generic
			setError(form, fieldName, message);
		}
	}
}
