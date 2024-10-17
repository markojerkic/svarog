import { useLocation, useSearchParams } from "@markojerkic/solid-router";
import { createMemo, on } from "solid-js";

export const useSelectedInstances = () => {
	const [searchParams] = useSearchParams();
	const selectedInstances = createMemo(
		on(
			() => useLocation().search,
			() => {
				return getArrayValueOfSearchParam(searchParams.instance);
			},
		),
	);
	return selectedInstances;
};

export const getArrayValueOfSearchParam = (
	searchParam: string | string[] | undefined,
) => {
	if (searchParam === undefined) {
		return [];
	}

	return Array.isArray(searchParam) ? searchParam : [searchParam];
};
