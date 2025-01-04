type ApiFieldErrors = Record<string, string>;
export type TApiError = { message: string; fields: ApiFieldErrors };

export class ApiError extends Error {
	constructor(public readonly apiError: TApiError) {
		super(apiError.message);
		console.error("ApiError from interceptor", apiError);
	}
}
