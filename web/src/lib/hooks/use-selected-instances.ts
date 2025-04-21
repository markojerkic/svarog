import { Route as logsRoute } from "@/routes/__authenticated/logs.$clientId.index";

export const useSelectedInstances = () => {
	const searchParams = logsRoute.useSearch();
	return searchParams().instances;
};

export const getArrayValueOfSearchParam = (
	searchParam: string | string[] | undefined,
) => {
	if (searchParam === undefined) {
		return [];
	}

	return Array.isArray(searchParam) ? searchParam : [searchParam];
};
