import type { SortFn } from "@/lib/store/sorted-list";
import type { LogLine } from "@/lib/hooks/use-log-store";

export const logsSortFn: SortFn<LogLine> = (a, b) => {
	const timestampDiff = a.timestamp - b.timestamp;
	if (timestampDiff !== 0) {
		return timestampDiff;
	}
	return a.sequenceNumber - b.sequenceNumber;
};
